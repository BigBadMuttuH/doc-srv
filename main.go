package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
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

const (
	exitCodeConfig         = 1
	exitCodeServiceControl = 2
	exitCodeRun            = 3
)

// Program structures.
// Define Start and Stop methods.
type program struct {
	server *http.Server
	cfg    Config

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
	p.rotWriter, err = newRotatingWriter(p.cfg.LogFile, maxLogSizeBytes)
	if err != nil {
		return err
	}
	accessLog = log.New(p.rotWriter, "", log.LstdFlags)

	// Doc Repository
	repo := NewDocRepository(p.cfg.DocsDir, p.cfg.CacheTTL)

	// Parse Template
	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
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

	// Health check endpoint
	mux.Handle("/healthz", healthHandler(p.cfg.DocsDir))

	// Handler - Serve documents
	docFS := http.FileServer(http.Dir(p.cfg.DocsDir))
	mux.Handle("/docs/", http.StripPrefix("/docs/", docFS))

	// Wrap mux with access logging middleware so that все запросы логируются единообразно.
	p.server = &http.Server{
		Addr:              ":" + p.cfg.Port,
		Handler:           loggingMiddleware(mux),
		ReadTimeout:       p.cfg.ReadTimeout,
		WriteTimeout:      p.cfg.WriteTimeout,
		IdleTimeout:       p.cfg.IdleTimeout,
		ReadHeaderTimeout: p.cfg.ReadHeaderTimeout,
	}

	// Start Server in goroutine
	go func() {
		log.Printf("Server starting on http://localhost:%s", p.cfg.Port)
		log.Printf("Serving documents from %s", p.cfg.DocsDir)
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

	if p.server != nil {
		if err := p.server.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
	}

	if p.rotWriter != nil {
		p.rotWriter.Close()
	}

	log.Println("Server exiting")
	return nil
}

func main() {
	// Flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	docsDirOverride := flag.String("dir", "", "Directory containing PDF files (overrides config)")
	portOverride := flag.String("port", "", "Server port (overrides config)")
	svcFlag := flag.String("service", "", "Control the system service: install, uninstall, start, stop")
	flag.Parse()

	// Load config (defaults + optional YAML file).
	cfg, err := LoadConfig(*configPath)
	if err != nil {
		log.Printf("failed to load config: %v", err)
		os.Exit(exitCodeConfig)
	}

	// Apply CLI overrides on top of config.
	if *docsDirOverride != "" {
		cfg.DocsDir = *docsDirOverride
	}
	if *portOverride != "" {
		cfg.Port = *portOverride
	}

	// Service configuration uses the same flags that were passed on install,
	// so SCM will restart the service with identical arguments.
	args := []string{"-config", *configPath}
	if *portOverride != "" {
		args = append(args, "-port", *portOverride)
	}
	if *docsDirOverride != "" {
		args = append(args, "-dir", *docsDirOverride)
	}

	svcConfig := &service.Config{
		Name:        "DocSrv",
		DisplayName: "Corporate Doc Server",
		Description: "HTTP server for serving PDF documents.",
		Arguments:   args,
	}

	prg := &program{
		cfg: cfg,
	}

	s, err := service.New(prg, svcConfig)
	if err != nil {
		log.Printf("failed to create service: %v", err)
		os.Exit(exitCodeConfig)
	}

	// Handle service controls
	if *svcFlag != "" {
		if err := service.Control(s, *svcFlag); err != nil {
			log.Printf("Valid actions: %q\nError: %s", service.ControlAction, err)
			os.Exit(exitCodeServiceControl)
		}
		return
	}

	// Run
	if err = s.Run(); err != nil {
		log.Printf("service run failed: %v", err)
		os.Exit(exitCodeRun)
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

// healthHandler проверяет доступность каталога документов и возвращает 200 OK,
// если всё в порядке. Используется для простого мониторинга сервиса.
func healthHandler(docsDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(docsDir); err != nil {
			http.Error(w, "docs directory is not accessible", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
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
			// /healthz обычно дергается очень часто мониторингом, поэтому
			// по умолчанию не логируем его, чтобы не засорять access.log.
			if r.URL.Path == "/healthz" {
				return
			}

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
