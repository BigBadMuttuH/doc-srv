package main

import (
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"sync"
	"time"
)

var (
	accessLog   *log.Logger
	accessLogMu sync.RWMutex
)

// setAccessLog обновляет глобальный accessLog под защитой мьютекса.
func setAccessLog(l *log.Logger) {
	accessLogMu.Lock()
	accessLog = l
	accessLogMu.Unlock()
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (lrw *loggingResponseWriter) WriteHeader(statusCode int) {
	lrw.status = statusCode
	lrw.ResponseWriter.WriteHeader(statusCode)
}

func (lrw *loggingResponseWriter) Write(p []byte) (int, error) {
	if lrw.status == 0 {
		// Если явно не вызывали WriteHeader, считаем статус 200.
		lrw.status = http.StatusOK
	}
	n, err := lrw.ResponseWriter.Write(p)
	lrw.bytes += n
	return n, err
}

// loggingMiddleware логирует все запросы в формате, близком к nginx combined log.
// /healthz не логируется, чтобы не засорять access.log частыми проверками мониторинга.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lrw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lrw, r)

		if r.URL.Path == "/healthz" {
			return
		}

		accessLogMu.RLock()
		logger := accessLog
		accessLogMu.RUnlock()

		if logger != nil {
			duration := time.Since(start)

			remote := r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				remote = host
			}

			logger.Printf("%s - - \"%s %s %s\" %d %d \"%s\" \"%s\" %.3f",
				remote,
				r.Method,
				r.URL.RequestURI(),
				r.Proto,
				lrw.status,
				lrw.bytes,
				r.Referer(),
				r.UserAgent(),
				duration.Seconds(),
			)
		}
	})
}

// recoveryMiddleware ловит паники в обработчиках и возвращает 500, не роняя сервер.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("PANIC: %v\n%s", rec, debug.Stack())
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
