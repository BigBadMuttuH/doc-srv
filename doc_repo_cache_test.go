package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Test that DocRepository caches results within TTL and refreshes after TTL expires.
func TestDocRepository_CacheTTL(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "doc_repo_cache_ttl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create one PDF file in root.
	filePath := filepath.Join(tmpDir, "doc1.pdf")
	if err := os.WriteFile(filePath, []byte("pdf"), 0644); err != nil {
		t.Fatal(err)
	}

	repo := NewDocRepository(tmpDir, 50*time.Millisecond)

	// First call populates cache.
	sections1, err := repo.GetSections()
	if err != nil {
		t.Fatalf("GetSections (first) failed: %v", err)
	}
	if len(sections1) != 1 {
		t.Fatalf("expected 1 section, got %d", len(sections1))
	}

	// Remove file; within TTL we still should see the old data from cache.
	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}

	sections2, err := repo.GetSections()
	if err != nil {
		t.Fatalf("GetSections (second) failed: %v", err)
	}
	if len(sections2) != 1 {
		t.Fatalf("expected cached 1 section within TTL, got %d", len(sections2))
	}

	// Wait for TTL to expire and expect cache refresh (no sections, так как файл удален).
	time.Sleep(60 * time.Millisecond)

	sections3, err := repo.GetSections()
	if err != nil {
		t.Fatalf("GetSections (third) failed: %v", err)
	}
	if len(sections3) != 0 {
		t.Fatalf("expected 0 sections after TTL expiration, got %d", len(sections3))
	}
}
