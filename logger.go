package main

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// simple size-based log rotation
const maxLogSizeBytes int64 = 10 * 1024 * 1024 // 10 MB

type rotatingWriter struct {
	filename string
	maxSize  int64
	file     *os.File
	size     int64
	mu       sync.Mutex
}

func newRotatingWriter(filename string, maxSize int64) (*rotatingWriter, error) {
	rw := &rotatingWriter{filename: filename, maxSize: maxSize}
	if err := rw.open(); err != nil {
		return nil, err
	}
	return rw, nil
}

// open must be called under lock or before server start
func (rw *rotatingWriter) open() error {
	f, err := os.OpenFile(rw.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	rw.file = f
	rw.size = info.Size()
	return nil
}

// rotate must be called under lock
func (rw *rotatingWriter) rotate() error {
	if rw.file != nil {
		// Ignore close error here as we are rotating
		_ = rw.file.Close()
		rw.file = nil
	}

	timestamp := time.Now().Format("20060102-150405")
	rotated := fmt.Sprintf("%s.%s", rw.filename, timestamp)
	if err := os.Rename(rw.filename, rotated); err != nil {
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	return rw.open()
}

func (rw *rotatingWriter) Write(p []byte) (int, error) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file == nil {
		if err := rw.open(); err != nil {
			return 0, err
		}
	}

	if rw.size+int64(len(p)) > rw.maxSize {
		if err := rw.rotate(); err != nil {
			// If rotation fails, we try to write to the current file anyway
			// or fallback to stderr if completely broken.
			// For now, just log to stderr that rotation failed
			fmt.Fprintf(os.Stderr, "log rotation failed: %v\n", err)
		}
	}

	n, err := rw.file.Write(p)
	rw.size += int64(n)
	return n, err
}

func (rw *rotatingWriter) Close() error {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if rw.file != nil {
		err := rw.file.Close()
		rw.file = nil
		return err
	}
	return nil
}
