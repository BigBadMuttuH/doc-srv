package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDocRepository_Scan(t *testing.T) {
	// Create temp dir
	tmpDir, err := os.MkdirTemp("", "doc_repo_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create root PDF
	if err := os.WriteFile(filepath.Join(tmpDir, "root.pdf"), []byte("pdf content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subdir
	subDir := filepath.Join(tmpDir, "HR")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create PDF in subdir
	if err := os.WriteFile(filepath.Join(subDir, "hiring.pdf"), []byte("pdf content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create README.md in subdir
	if err := os.WriteFile(filepath.Join(subDir, "README.md"), []byte("# HR Section"), 0644); err != nil {
		t.Fatal(err)
	}

	// Initialize Repo
	repo := NewDocRepository(tmpDir, time.Minute)

	// Get Sections
	sections, err := repo.GetSections()
	if err != nil {
		t.Fatalf("GetSections failed: %v", err)
	}

	// Verify
	// We expect 2 sections: "Общее" (root) and "HR"
	if len(sections) != 2 {
		t.Errorf("expected 2 sections, got %d", len(sections))
	}

	// Check Root section
	foundGeneral := false
	foundHR := false

	for _, s := range sections {
		switch s.Name {
		case "Общее":
			foundGeneral = true
			if len(s.Documents) != 1 {
				t.Errorf("expected 1 document in General, got %d", len(s.Documents))
			}
			if s.Documents[0].Name != "root.pdf" {
				t.Errorf("expected root.pdf, got %s", s.Documents[0].Name)
			}
		case "HR":
			foundHR = true
			if len(s.Documents) != 1 {
				t.Errorf("expected 1 document in HR, got %d", len(s.Documents))
			}
			if s.Documents[0].Name != "hiring.pdf" {
				t.Errorf("expected hiring.pdf, got %s", s.Documents[0].Name)
			}
			// Check Readme presence
			if s.Readme == "" {
				t.Error("expected Readme content, got empty")
			}
		}
	}

	if !foundGeneral {
		t.Error("General section not found")
	}
	if !foundHR {
		t.Error("HR section not found")
	}
}
