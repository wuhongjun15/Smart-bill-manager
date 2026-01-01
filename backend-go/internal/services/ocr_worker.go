package services

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type rapidOCRWorkerProcess struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	waitCh chan error
}

var globalRapidOCRWorker rapidOCRWorkerProcess

func ocrWorkerEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("SBM_OCR_WORKER")))
	return v == "1" || v == "true" || v == "yes" || v == "y" || v == "on"
}

func StartOCRWorkerIfEnabled() (bool, error) {
	if !ocrWorkerEnabled() {
		return false, nil
	}
	svc := NewOCRService()
	scriptPath := svc.findOCRWorkerScript()
	if strings.TrimSpace(scriptPath) == "" {
		return false, fmt.Errorf("ocr worker enabled but scripts/ocr_worker.py not found")
	}
	globalRapidOCRWorker.mu.Lock()
	defer globalRapidOCRWorker.mu.Unlock()
	if err := globalRapidOCRWorker.ensureStartedLocked(scriptPath); err != nil {
		return false, err
	}
	return true, nil
}

func randHex(nBytes int) string {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func parseOCRProfileFromArgs(extraArgs []string) string {
	profile := "default"
	for i := 0; i < len(extraArgs); i++ {
		if strings.TrimSpace(extraArgs[i]) == "--profile" && i+1 < len(extraArgs) {
			p := strings.TrimSpace(extraArgs[i+1])
			if p != "" {
				profile = p
			}
			break
		}
	}
	if profile != "default" && profile != "pdf" {
		profile = "default"
	}
	return profile
}

func (s *OCRService) findOCRWorkerScript() string {
	locations := []string{
		"scripts/ocr_worker.py",
		"../scripts/ocr_worker.py",
		"/app/scripts/ocr_worker.py",
		"./ocr_worker.py",
	}
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}
	return ""
}

func (w *rapidOCRWorkerProcess) isRunningLocked() bool {
	if w.cmd == nil || w.waitCh == nil {
		return false
	}
	select {
	case <-w.waitCh:
		return false
	default:
		return true
	}
}

func (w *rapidOCRWorkerProcess) stopLocked() {
	if w.stdin != nil {
		_ = w.stdin.Close()
		w.stdin = nil
	}
	if w.cmd != nil && w.cmd.Process != nil {
		_ = w.cmd.Process.Kill()
	}
	w.cmd = nil
	w.stdout = nil
	w.waitCh = nil
}

func (w *rapidOCRWorkerProcess) startLocked(python string, scriptPath string) error {
	cmd := exec.Command(python, scriptPath)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Drain stderr to avoid blocking the child if it writes a lot.
	go func() {
		_, _ = io.Copy(io.Discard, stderrPipe)
	}()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	w.cmd = cmd
	w.stdin = stdinPipe
	w.stdout = bufio.NewReaderSize(stdoutPipe, 1024*1024)
	w.waitCh = waitCh
	return nil
}

func (w *rapidOCRWorkerProcess) ensureStartedLocked(scriptPath string) error {
	if w.isRunningLocked() {
		return nil
	}
	w.stopLocked()

	var lastErr error
	for _, python := range []string{"python3", "python"} {
		if err := w.startLocked(python, scriptPath); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("failed to start ocr worker")
	}
	return lastErr
}

type ocrWorkerBaseResponse struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

func (w *rapidOCRWorkerProcess) recognizeLocked(scriptPath string, imagePath string, profile string) ([]byte, error) {
	if err := w.ensureStartedLocked(scriptPath); err != nil {
		return nil, err
	}

	reqID := randHex(12)
	req := map[string]any{
		"id":         reqID,
		"type":       "ocr",
		"image_path": imagePath,
		"profile":    profile,
	}
	b, _ := json.Marshal(req)
	b = append(b, '\n')

	if _, err := w.stdin.Write(b); err != nil {
		w.stopLocked()
		return nil, fmt.Errorf("ocr worker write failed: %w", err)
	}

	type readRes struct {
		line []byte
		err  error
	}
	ch := make(chan readRes, 1)
	go func() {
		line, err := w.stdout.ReadBytes('\n')
		ch <- readRes{line: line, err: err}
	}()

	select {
	case r := <-ch:
		if r.err != nil {
			w.stopLocked()
			return nil, fmt.Errorf("ocr worker read failed: %w", r.err)
		}
		line := bytes.TrimSpace(r.line)
		if len(line) == 0 {
			return nil, fmt.Errorf("ocr worker returned empty response")
		}

		var base ocrWorkerBaseResponse
		if err := unmarshalPossiblyNoisyJSON(line, &base); err != nil {
			return nil, fmt.Errorf("ocr worker returned invalid json: %w", err)
		}
		if strings.TrimSpace(base.ID) != reqID {
			return nil, fmt.Errorf("ocr worker response id mismatch")
		}
		if !base.Success {
			if strings.TrimSpace(base.Error) == "" {
				return nil, fmt.Errorf("ocr worker failed")
			}
			return nil, fmt.Errorf("ocr worker failed: %s", strings.TrimSpace(base.Error))
		}

		return line, nil
	case <-time.After(rapidOCRTimeout + 10*time.Second):
		w.stopLocked()
		return nil, fmt.Errorf("ocr worker timeout")
	}
}

func recognizeWithRapidOCRWorker(scriptPath string, imagePath string, profile string) ([]byte, error) {
	globalRapidOCRWorker.mu.Lock()
	defer globalRapidOCRWorker.mu.Unlock()
	return globalRapidOCRWorker.recognizeLocked(scriptPath, imagePath, profile)
}
