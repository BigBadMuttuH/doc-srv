package main

import (
	"context"
	"embed"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed templates/index.html static/*
var content embed.FS

var accessLog *log.Logger

func main() {
	// Config
	docsDir := flag.String("dir", "./docs", "Directory containing PDF files")
	port := flag.String("port", "8080", "Server port")
	flag.Parse()

	// Access log with rotation
	rotWriter, err := newRotatingWriter("access.log", maxLogSizeBytes)
	if err != nil {
		log.Fatalf("Could not initialize access.log: %v", err)
	}
	defer rotWriter.Close()
	accessLog = log.New(rotWriter, "", log.LstdFlags)

	// Doc Repository
	repo := NewDocRepository(*docsDir, 5*time.Minute)

	// Parse Template
	tmpl, err := template.ParseFS(content, "templates/index.html")
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	// Handler - List
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
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

		if accessLog != nil {
			accessLog.Printf("INDEX remote=%s ua=%q", r.RemoteAddr, r.UserAgent())
		}
	})

	// Handler - Static (CSS)
	staticServer := http.FileServer(http.FS(content))
	http.Handle("/static/", staticServer)

	// Handler - Serve documents with logging
	docFS := http.FileServer(http.Dir(*docsDir))
	http.Handle("/docs/", http.StripPrefix("/docs/", loggingMiddleware(docFS)))

	// Server
	srv := &http.Server{
		Addr:    ":" + *port,
		Handler: nil, // DefaultServeMux
	}

	// Graceful Shutdown
	go func() {
		log.Printf("Server starting on http://localhost:%s", *port)
		log.Printf("Serving documents from %s", *docsDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if accessLog != nil {
			accessLog.Printf("DOC remote=%s method=%s path=%s", r.RemoteAddr, r.Method, r.URL.Path)
		}
		next.ServeHTTP(w, r)
	})
}
