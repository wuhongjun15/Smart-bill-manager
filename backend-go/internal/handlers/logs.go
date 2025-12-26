package handlers

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type LogsHandler struct {
	sources map[string]string // name -> path
}

func NewLogsHandler() *LogsHandler {
	return &LogsHandler{
		sources: defaultLogSources(),
	}
}

func (h *LogsHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/sources", h.ListSources)
	r.GET("/tail", h.Tail)
	r.GET("/stream", h.Stream)
}

type LogSource struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

type LogEvent struct {
	Type      string `json:"type"` // "log" | "ping"
	Timestamp string `json:"timestamp"`
	Source    string `json:"source,omitempty"`
	Message   string `json:"message,omitempty"`
}

func defaultLogSources() map[string]string {
	if raw := strings.TrimSpace(os.Getenv("LOG_SOURCES")); raw != "" {
		// Format: name=path,name2=path2
		// Example: backend_out=/var/log/supervisor/backend.out.log,backend_err=/var/log/supervisor/backend.err.log
		out := make(map[string]string)
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			name := strings.TrimSpace(kv[0])
			path := strings.TrimSpace(kv[1])
			if name == "" || path == "" {
				continue
			}
			out[name] = path
		}
		if len(out) > 0 {
			return out
		}
	}

	// Supervisor log files inside the container.
	// These are referenced by supervisord.conf.
	return map[string]string{
		"backend_out": "/var/log/supervisor/backend.out.log",
		"backend_err": "/var/log/supervisor/backend.err.log",
		"nginx_out":   "/var/log/supervisor/nginx.out.log",
		"nginx_err":   "/var/log/supervisor/nginx.err.log",
	}
}

func (h *LogsHandler) ListSources(c *gin.Context) {
	var out []LogSource
	for name, path := range h.sources {
		_, err := os.Stat(path)
		out = append(out, LogSource{Name: name, Available: err == nil})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	c.JSON(200, gin.H{"success": true, "data": out})
}

func (h *LogsHandler) Tail(c *gin.Context) {
	names := parseCSV(c.Query("sources"))
	if len(names) == 0 {
		names = []string{"backend_out", "backend_err"}
	}
	maxLines := parseIntBounded(c.Query("lines"), 200, 1, 2000)

	var events []LogEvent
	now := time.Now().UTC().Format(time.RFC3339Nano)

	for _, name := range names {
		path, ok := h.sources[name]
		if !ok {
			continue
		}
		lines, err := tailFileLines(path, maxLines, 512*1024)
		if err != nil {
			continue
		}
		for _, line := range lines {
			events = append(events, LogEvent{
				Type:      "log",
				Timestamp: now,
				Source:    name,
				Message:   line,
			})
		}
	}

	c.JSON(200, gin.H{"success": true, "data": events})
}

func (h *LogsHandler) Stream(c *gin.Context) {
	names := parseCSV(c.Query("sources"))
	if len(names) == 0 {
		names = []string{"backend_out", "backend_err"}
	}

	paths := make(map[string]string)
	for _, name := range names {
		if path, ok := h.sources[name]; ok {
			paths[name] = path
		}
	}
	if len(paths) == 0 {
		c.JSON(400, gin.H{"success": false, "message": "no valid log sources"})
		return
	}

	c.Writer.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.WriteHeader(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(500, gin.H{"success": false, "message": "streaming not supported"})
		return
	}

	// Start from end by default, so opening the view doesn't dump huge logs.
	offsets := make(map[string]int64)
	remainders := make(map[string]string)
	for name, path := range paths {
		if fi, err := os.Stat(path); err == nil {
			offsets[name] = fi.Size()
		} else {
			offsets[name] = 0
		}
		remainders[name] = ""
	}

	enc := json.NewEncoder(c.Writer)
	enc.SetEscapeHTML(false)
	writeEvent := func(evt LogEvent) error { return enc.Encode(evt) }

	ticker := time.NewTicker(800 * time.Millisecond)
	defer ticker.Stop()

	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-pingTicker.C:
			_ = writeEvent(LogEvent{
				Type:      "ping",
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			})
			flusher.Flush()
		case <-ticker.C:
			for name, path := range paths {
				newOffset, remainder, lines := readNewLines(path, offsets[name], remainders[name], 256*1024)
				offsets[name] = newOffset
				remainders[name] = remainder
				for _, line := range lines {
					_ = writeEvent(LogEvent{
						Type:      "log",
						Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
						Source:    name,
						Message:   line,
					})
				}
				flusher.Flush()
			}
		}
	}
}

func parseCSV(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseIntBounded(v string, def, min, max int) int {
	if strings.TrimSpace(v) == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if n < min {
		return min
	}
	if n > max {
		return max
	}
	return n
}

func tailFileLines(path string, maxLines int, maxBytes int64) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}

	size := fi.Size()
	start := int64(0)
	if size > maxBytes {
		start = size - maxBytes
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}

	var lines []string
	scanner := bufio.NewScanner(f)
	// Allow long log lines.
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return lines, nil
}

func readNewLines(path string, offset int64, remainder string, maxRead int64) (int64, string, []string) {
	f, err := os.Open(path)
	if err != nil {
		return offset, remainder, nil
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return offset, remainder, nil
	}
	size := fi.Size()
	// Handle truncation/rotation.
	if size < offset {
		offset = 0
		remainder = ""
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, remainder, nil
	}

	// Limit read to avoid unbounded memory usage if log bursts.
	limited := io.LimitReader(f, maxRead)
	b, err := io.ReadAll(limited)
	if err != nil {
		return offset, remainder, nil
	}
	newOffset := offset + int64(len(b))
	if len(b) == 0 {
		return newOffset, remainder, nil
	}

	combined := remainder + string(b)
	parts := strings.Split(combined, "\n")

	// If the stream doesn't end with newline, keep the last part as remainder.
	if !strings.HasSuffix(combined, "\n") {
		remainder = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	} else {
		remainder = ""
	}

	var lines []string
	for _, p := range parts {
		p = strings.TrimRight(p, "\r")
		if strings.TrimSpace(p) == "" {
			continue
		}
		lines = append(lines, p)
	}

	return newOffset, remainder, lines
}
