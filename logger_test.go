package main

import (
	"os"
	"testing"
)

func TestRotatingWriter(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_log_*.log")
	if err != nil {
		t.Fatal(err)
	}
	filename := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(filename)

	// Small max size to trigger rotation
	rw, err := newRotatingWriter(filename, 20)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	defer rw.Close()

	// Write some data
	data := []byte("1234567890") // 10 bytes
	if _, err := rw.Write(data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Write more data to trigger rotation (10 + 10 = 20 >= maxSize)
	// Wait, condition is rw.size + len(p) > rw.maxSize
	// 10 + 10 = 20. If maxSize is 20, 20 > 20 is false.
	// So second write fits.
	if _, err := rw.Write(data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Third write should trigger rotation
	if _, err := rw.Write(data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Check if rotated file exists
	// We can't easily predict the rotated filename timestamp.
	// But we can check if the current file is small (just 10 bytes from 3rd write).

	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}

	if info.Size() != 10 {
		t.Errorf("expected current file size 10, got %d", info.Size())
	}
}
