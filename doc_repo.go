package main

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
)

type Document struct {
	Name string
	URL  string
}

type Section struct {
	Name      string
	Documents []Document
	Readme    template.HTML
}

type DocRepository struct {
	dir       string
	cache     []Section
	cacheTime time.Time
	mu        sync.RWMutex
	ttl       time.Duration
}

func NewDocRepository(dir string, cacheTTL time.Duration) *DocRepository {
	return &DocRepository{
		dir: dir,
		ttl: cacheTTL,
	}
}

func (r *DocRepository) GetSections() ([]Section, error) {
	r.mu.RLock()
	if time.Since(r.cacheTime) < r.ttl && r.cache != nil {
		defer r.mu.RUnlock()
		return r.cache, nil
	}
	r.mu.RUnlock()

	// Cache expired or empty, refresh
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check locking
	if time.Since(r.cacheTime) < r.ttl && r.cache != nil {
		return r.cache, nil
	}

	sections, err := r.scan()
	if err != nil {
		return nil, err
	}

	r.cache = sections
	r.cacheTime = time.Now()
	return sections, nil
}

func (r *DocRepository) scan() ([]Section, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil, fmt.Errorf("could not read docs directory: %w", err)
	}

	var sections []Section

	// 1. Root level files (General section)
	var generalDocs []Document
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".pdf") {
			generalDocs = append(generalDocs, Document{
				Name: e.Name(),
				URL:  "/docs/" + e.Name(),
			})
		}
	}
	if len(generalDocs) > 0 {
		sections = append(sections, Section{Name: "Общее", Documents: generalDocs})
	}

	// 2. Subdirectories (Named sections)
	for _, e := range entries {
		if e.IsDir() {
			subDirName := e.Name()
			subDirPath := filepath.Join(r.dir, subDirName)
			subEntries, err := os.ReadDir(subDirPath)
			if err != nil {
				log.Printf("Error reading subdirectory %s: %v", subDirName, err)
				continue
			}

			var subDocs []Document
			var readmeContent template.HTML

			for _, subE := range subEntries {
				if !subE.IsDir() {
					lowerName := strings.ToLower(subE.Name())
					if strings.HasSuffix(lowerName, ".pdf") {
						subDocs = append(subDocs, Document{
							Name: subE.Name(),
							URL:  "/docs/" + subDirName + "/" + subE.Name(),
						})
					} else if lowerName == "readme.md" {
						// Read README content
						content, err := os.ReadFile(filepath.Join(subDirPath, subE.Name()))
						if err == nil {
							var buf bytes.Buffer
							if err := goldmark.Convert(content, &buf); err == nil {
								readmeContent = template.HTML(buf.String())
							} else {
								readmeContent = template.HTML(template.HTMLEscapeString(string(content)))
							}
						}
					}
				}
			}

			if len(subDocs) > 0 || readmeContent != "" {
				sections = append(sections, Section{
					Name:      subDirName,
					Documents: subDocs,
					Readme:    readmeContent,
				})
			}
		}
	}

	return sections, nil
}
