package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type RotatingLogger struct {
	mu         sync.Mutex
	logDir     string
	maxSize    int64
	maxBackups int
	currentDay string
	currentFile *os.File
	fileSize   int64
}

func NewRotatingLogger(logDir string, maxSize int64, maxBackups int) *RotatingLogger {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Fatalf("Failed to create log directory: %v", err)
	}
	rl := &RotatingLogger{
		logDir:     logDir,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		currentDay: time.Now().Format("2006-01-02"),
	}
	if err := rl.openFile(); err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	go rl.checkDayRotation()
	return rl
}

func (rl *RotatingLogger) checkDayRotation() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		day := time.Now().Format("2006-01-02")
		if day != rl.currentDay {
			rl.rotateByDay()
		}
	}
}

func (rl *RotatingLogger) rotateByDay() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.currentDay = time.Now().Format("2006-01-02")
	if rl.currentFile != nil {
		rl.currentFile.Sync()
		rl.currentFile.Close()
	}
	rl.fileSize = 0
	if err := rl.openFile(); err != nil {
		log.Printf("Failed to rotate log by day: %v", err)
	}
}

func (rl *RotatingLogger) openFile() error {
	filePath := filepath.Join(rl.logDir, fmt.Sprintf("%s.0.log", rl.currentDay))
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	rl.currentFile = f
	rl.fileSize = info.Size()
	return nil
}

func (rl *RotatingLogger) checkSizeRotation() error {
	if rl.fileSize < rl.maxSize {
		return nil
	}
	if rl.currentFile != nil {
		rl.currentFile.Sync()
		rl.currentFile.Close()
	}
	rl.currentFile = nil
	rl.fileSize = 0
	base := filepath.Join(rl.logDir, fmt.Sprintf("%s.", rl.currentDay))
	for i := rl.maxBackups - 1; i >= 1; i-- {
		old := fmt.Sprintf("%s%d.log", base, i)
		new := fmt.Sprintf("%s%d.log", base, i+1)
		os.Rename(old, new)
	}
	oldName := fmt.Sprintf("%s0.log", base)
	newName := fmt.Sprintf("%s1.log", base)
	os.Rename(oldName, newName)
	return rl.openFile()
}

func (rl *RotatingLogger) Write(data []string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if err := rl.checkSizeRotation(); err != nil {
		return err
	}
	for _, line := range data {
		timestamp := time.Now().Format("2006-01-02 15:04:05.000")
		msg := fmt.Sprintf("[%s] %s\n", timestamp, line)
		n, err := rl.currentFile.WriteString(msg)
		if err != nil {
			return err
		}
		rl.fileSize += int64(n)
	}
	return rl.currentFile.Sync()
}

func (rl *RotatingLogger) Close() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.currentFile != nil {
		rl.currentFile.Sync()
		rl.currentFile.Close()
	}
}

func handleLogs(rl *RotatingLogger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}
		content := strings.TrimSpace(string(buf))
		if content == "" {
			http.Error(w, "Empty log content", http.StatusBadRequest)
			return
		}
		lines := strings.Split(content, "\n")
		if err := rl.Write(lines); err != nil {
			http.Error(w, "Failed to write log", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "OK")
	}
}

func main() {
	logDir := "logs"
	maxSize := int64(64 * 1024 * 1024)
	maxBackups := 10

	rl := NewRotatingLogger(logDir, maxSize, maxBackups)
	defer rl.Close()

	http.HandleFunc("/logs", handleLogs(rl))
	addr := "0.0.0.0:9554"
	log.Printf("Log server started on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
