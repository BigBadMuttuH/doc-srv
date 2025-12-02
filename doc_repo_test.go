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

	// Create nested subdir HR/2025
	yearDir := filepath.Join(subDir, "2025")
	if err := os.Mkdir(yearDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create PDF in nested subdir
	if err := os.WriteFile(filepath.Join(yearDir, "plan.pdf"), []byte("pdf content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create README.md in nested subdir
	if err := os.WriteFile(filepath.Join(yearDir, "README.md"), []byte("# HR 2025"), 0644); err != nil {
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
	// We expect 3 sections: "Общее" (root), "HR" и "HR/2025"
	if len(sections) != 3 {
		t.Errorf("expected 3 sections, got %d", len(sections))
	}

	// Check sections
	foundGeneral := false
	foundHR := false
	foundHR2025 := false

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
				t.Error("expected Readme content in HR, got empty")
			}
		case "HR/2025":
			foundHR2025 = true
			if len(s.Documents) != 1 {
				t.Errorf("expected 1 document in HR/2025, got %d", len(s.Documents))
			}
			if s.Documents[0].Name != "plan.pdf" {
				t.Errorf("expected plan.pdf, got %s", s.Documents[0].Name)
			}
			if s.Readme == "" {
				t.Error("expected Readme content in HR/2025, got empty")
			}
		}
	}

	if !foundGeneral {
		t.Error("General section not found")
	}
	if !foundHR {
		t.Error("HR section not found")
	}
	if !foundHR2025 {
		t.Error("HR/2025 section not found")
	}
}
