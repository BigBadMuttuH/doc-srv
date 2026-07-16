package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/kardianos/service"
)

//go:embed templates/index.html static/*
var content embed.FS

const (
	exitCodeConfig         = 1
	exitCodeServiceControl = 2
	exitCodeRun            = 3
)

// Program structures.
// Define Start and Stop methods.
type program struct {
	server   *http.Server
	cfg      Config
	rotWriter *rotatingWriter

	configPath    string // путь к config.yaml (для перезагрузки после chdir)
	portOverride  string // CLI-флаг -port (сохраняем, чтобы не затёрся перезагрузкой)
	dirOverride   string // CLI-флаг -dir (сохраняем, чтобы не затёрся перезагрузкой)
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

		// Перезагружаем конфиг: main() грузил его до смены рабочей директории,
		// поэтому config.yaml мог не найтись (CWD = System32, а не папка exe).
		if cfg, err := LoadConfig(p.configPath); err == nil {
			p.cfg = cfg
			// CLI-переопределения (port, dir) поверх файла конфига.
			if p.portOverride != "" {
				p.cfg.Port = p.portOverride
			}
			if p.dirOverride != "" {
				p.cfg.DocsDir = p.dirOverride
			}
		} else {
			log.Printf("Warning: failed to reload config after chdir: %v", err)
		}
	}

	// Initialize Logging
	var err error
	p.rotWriter, err = newRotatingWriter(p.cfg.LogFile, maxLogSizeBytes)
	if err != nil {
		return err
	}
	setAccessLog(log.New(p.rotWriter, "", log.LstdFlags))

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
		if err := tmpl.Execute(w, pageData{Sections: sections, OrgName: p.cfg.OrgName}); err != nil {
			log.Printf("Error executing template: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
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

	// Wrap mux with recovery + access logging middleware.
	p.server = &http.Server{
		Addr:              ":" + p.cfg.Port,
		Handler:           loggingMiddleware(recoveryMiddleware(mux)),
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

// version переопределяется при сборке флагом -ldflags -X main.version=x.y.z
var version = "dev"

func main() {
	// Flags
	configPath := flag.String("config", "config.yaml", "Path to config file")
	docsDirOverride := flag.String("dir", "", "Directory containing PDF files (overrides config)")
	portOverride := flag.String("port", "", "Server port (overrides config)")
	svcFlag := flag.String("service", "", "Control the system service: install, uninstall, start, stop")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("doc-srv version %s\n", version)
		return
	}

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
		cfg:          cfg,
		configPath:   *configPath,
		portOverride: *portOverride,
		dirOverride:  *docsDirOverride,
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

// pageData передаётся в HTML-шаблон главной страницы.
type pageData struct {
	Sections []Section
	OrgName  string
}


