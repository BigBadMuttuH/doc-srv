package main

import (
	"net/http"
	"os"
)

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
