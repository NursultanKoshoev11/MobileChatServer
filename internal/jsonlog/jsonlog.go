package jsonlog

import (
	"encoding/json"
	"io"
	"strings"
	"sync"
	"time"
)

type Writer struct {
	out io.Writer
	mu  sync.Mutex
}

func New(out io.Writer) *Writer {
	return &Writer{out: out}
}

func (w *Writer) Write(p []byte) (int, error) {
	entry := map[string]any{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"level":   "info",
		"message": strings.TrimSpace(string(p)),
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return 0, err
	}
	line = append(line, '\n')
	w.mu.Lock()
	defer w.mu.Unlock()
	_, err = w.out.Write(line)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}
