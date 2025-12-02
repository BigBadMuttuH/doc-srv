package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
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

// scan выполняет рекурсивный обход каталога документов.
//
//  * Файлы .pdf в корне r.dir попадают в секцию "Общее".
//  * Каждая поддиректория (любого уровня), в которой есть хотя бы один .pdf
//    или README.md, становится отдельной секцией с именем вида "HR/2025".
//  * README.md в каждой директории рендерится в HTML, а относительные
//    ссылки/картинки переписываются на базу "/docs/<relative-dir>/...".
func (r *DocRepository) scan() ([]Section, error) {
	// Проверим, что корневая директория доступна.
	if _, err := os.Stat(r.dir); err != nil {
		return nil, fmt.Errorf("could not stat docs directory: %w", err)
	}

	var (
		sections    []Section
		generalDocs []Document

		// Ключ - относительный путь директории (с файловыми разделителями),
		// значение - собираемая секция.
		sectionsMap = make(map[string]*Section)
	)

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("Error accessing %s: %v", path, err)
			return nil // пропускаем проблемные узлы, но не останавливаем обход
		}

		// Корневую директорию пропускаем, нас интересуют только файлы/поддиректории.
		if path == r.dir {
			return nil
		}

		rel, err := filepath.Rel(r.dir, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Секцию создадим лениво, когда найдём файлы/README.
			return nil
		}

		lowerName := strings.ToLower(d.Name())
		dirRel := filepath.Dir(rel) // относительный путь директории

		// Файлы в корне r.dir → секция "Общее".
		if dirRel == "." {
			if strings.HasSuffix(lowerName, ".pdf") {
				generalDocs = append(generalDocs, Document{
					Name: d.Name(),
					URL:  "/docs/" + d.Name(),
				})
			}
			return nil
		}

		// Все остальные файлы относятся к некоторой поддиректории.
		sec, ok := sectionsMap[dirRel]
		if !ok {
			sec = &Section{Name: filepath.ToSlash(dirRel)}
			sectionsMap[dirRel] = sec
		}

		if strings.HasSuffix(lowerName, ".pdf") {
			// Собираем URL по относительному пути внутри /docs/.
			sec.Documents = append(sec.Documents, Document{
				Name: d.Name(),
				URL:  "/docs/" + filepath.ToSlash(rel),
			})
			return nil
		}

		if lowerName == "readme.md" {
			readmeHTML, err := renderReadme(path, filepath.ToSlash(dirRel))
			if err != nil {
				log.Printf("Error reading README in %s: %v", dirRel, err)
				return nil
			}
			sec.Readme = readmeHTML
		}

		return nil
	}

	if err := filepath.WalkDir(r.dir, walkFn); err != nil {
		return nil, fmt.Errorf("could not walk docs directory: %w", err)
	}

	// Собираем итоговый срез секций.
	if len(generalDocs) > 0 {
		// Отсортируем документы в "Общее" по имени.
		sort.Slice(generalDocs, func(i, j int) bool {
			return strings.ToLower(generalDocs[i].Name) < strings.ToLower(generalDocs[j].Name)
		})
		sections = append(sections, Section{Name: "Общее", Documents: generalDocs})
	}

	// Секции из поддиректорий: сортируем по имени секции и по имени документа внутри.
	keys := make([]string, 0, len(sectionsMap))
	for k := range sectionsMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		sec := sectionsMap[k]
		if len(sec.Documents) == 0 && sec.Readme == "" {
			continue
		}

		sort.Slice(sec.Documents, func(i, j int) bool {
			return strings.ToLower(sec.Documents[i].Name) < strings.ToLower(sec.Documents[j].Name)
		})

		sections = append(sections, *sec)
	}

	return sections, nil
}

// renderReadme читает README.md по заданному пути и рендерит его в HTML,
// переписывая относительные ссылки/картинки на базу "/docs/<relDir>/".
// relDir - относительный путь директории внутри r.dir, в формате с "/".
func renderReadme(path string, relDir string) (template.HTML, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	md := goldmark.New()
	ctx := parser.NewContext()

	reader := text.NewReader(content)
	doc := md.Parser().Parse(reader, parser.WithContext(ctx))

	basePrefix := "/docs/" + relDir + "/"

	// Проходим по AST и переписываем относительные URL.
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		rewrite := func(dest []byte) []byte {
			url := string(dest)
			if isRelativeURL(url) {
				return []byte(basePrefix + url)
			}
			return dest
		}

		if img, ok := n.(*ast.Image); ok {
			img.Destination = rewrite(img.Destination)
		}
		if link, ok := n.(*ast.Link); ok {
			link.Destination = rewrite(link.Destination)
		}

		return ast.WalkContinue, nil
	})

	var buf bytes.Buffer
	if err := md.Renderer().Render(&buf, content, doc); err != nil {
		// Fallback: эскейпнем исходный Markdown как текст, если рендеринг не удался.
		return template.HTML(template.HTMLEscapeString(string(content))), nil
	}

	return template.HTML(buf.String()), nil
}

// isRelativeURL возвращает true, если URL выглядит как относительный путь
// внутри README (не начинается с '/' или http/https).
func isRelativeURL(u string) bool {
	lower := strings.ToLower(u)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return false
	}
	return !strings.HasPrefix(lower, "/")
}
