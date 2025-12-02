package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kardianos/service"
)

//go:embed templates/index.html static/*
var content embed.FS

var accessLog *log.Logger

// Program structures.
// Define Start and Stop methods.
type program struct {
	server    *http.Server
	port      string
	docsDir   string
	rotWriter *rotatingWriter
}

func (p *program) Start(s service.Service) error {
	// Set working directory to the same directory as the executable
	// so that "docs" and logs are found correctly relative to the exe.
	if service.Interactive() {
		log.Printf("Running in interactive mode")
	} else {
		log.Printf("Running as service")
		if exePath, err := os.Executable(); err == nil {
			if err := os.Chdir(filepath.Dir(exePath)); err != nil {
				log.Printf("Failed to change directory: %v", err)
			}
		}
	}

	// Initialize Logging
	var err error
	p.rotWriter, err = newRotatingWriter("access.log", maxLogSizeBytes)
	if err != nil {
		return err
	}
	accessLog = log.New(p.rotWriter, "", log.LstdFlags)

	// Doc Repository
	repo := NewDocRepository(p.docsDir, 5*time.Minute)

	// Parse Template
	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Handlers
	mux := http.NewServeMux()

	// Handler - List
	indexHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		sections, err := repo.GetSections()
		if err != nil {
			http.Error(w, "Could not load documents", http.StatusInternalServerError)
			log.Printf("Error getting sections: %v", err)
			return
		}

		w.Header().Set("Content-Type", "text/html")
		if err := tmpl.Execute(w, sections); err != nil {
			log.Printf("Error executing template: %v", err)
			return
		}
	})
	mux.Handle("/", indexHandler)

	// Handler - Static (CSS)
	staticServer := http.FileServer(http.FS(content))
	mux.Handle("/static/", staticServer)

	// Handler - Serve documents
	docFS := http.FileServer(http.Dir(p.docsDir))
	mux.Handle("/docs/", http.StripPrefix("/docs/", docFS))

	// Wrap mux with access logging middleware so that все запросы логируются единообразно.
	// HTTP server with sane defaults for timeouts to protect от висящих соединений.
	p.server = &http.Server{
		Addr:         ":" + p.port,
		Handler:      loggingMiddleware(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second, ReadHeaderTimeout: 5 * time.Second,
	}

	// Start Server in goroutine
	go func() {
		log.Printf("Server starting on http://localhost:%s", p.port)
		log.Printf("Serving documents from %s", p.docsDir)
		if err := p.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Listen error: %v", err)
		}
	}()

	return nil
}

func (p *program) Stop(s service.Service) error {
	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	if p.rotWriter != nil {
		p.rotWriter.Close()
	}

	log.Println("Server exiting")
	return nil
}

func main() {
	// Config
	docsDir := flag.String("dir", "./docs", "Directory containing PDF files")
	port := flag.String("port", "8080", "Server port")
	svcFlag := flag.String("service", "", "Control the system service: install, uninstall, start, stop")
	flag.Parse()

	svcConfig := &service.Config{
		Name:        "DocSrv",
		DisplayName: "Corporate Doc Server",
		Description: "HTTP server for serving PDF documents.",
		Arguments:   []string{"-port", *port, "-dir", *docsDir},
	}

	prg := &program{
		port:    *port,
		docsDir: *docsDir,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	// Handle service controls
	if *svcFlag != "" {
		err = service.Control(s, *svcFlag)
		if err != nil {
			log.Fatalf("Valid actions: %q\nError: %s", service.ControlAction, err)
		}
		return
	}

	// Run
	if err = s.Run(); err != nil {
		log.Fatal(err)
	}
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lrw := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(lrw, r)

		if accessLog != nil {
			duration := time.Since(start)

			remote := r.RemoteAddr
			if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
				remote = host
			}

			// Формат, близкий к nginx combined log (без времени, его пишет log.Logger):
			// $remote_addr - - "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent" $request_time
			accessLog.Printf("%s - - \"%s %s %s\" %d %d \"%s\" \"%s\" %.3f",
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
