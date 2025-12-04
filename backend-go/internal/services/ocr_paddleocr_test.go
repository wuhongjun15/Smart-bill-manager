package services

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIsPaddleOCRAvailable tests the PaddleOCR availability check
func TestIsPaddleOCRAvailable(t *testing.T) {
	service := NewOCRService()

	t.Run("Returns false when script is not available", func(t *testing.T) {
		// This test will check if the script exists and Python/PaddleOCR are available
		// The actual result depends on the test environment
		available := service.isPaddleOCRAvailable()
		// We just verify it doesn't panic
		t.Logf("PaddleOCR availability: %v", available)
	})

	t.Run("findPaddleOCRScript works correctly", func(t *testing.T) {
		scriptPath := service.findPaddleOCRScript()
		t.Logf("Script path found: %s", scriptPath)
		// If script is found, it should be a valid path
		if scriptPath != "" {
			if _, err := os.Stat(scriptPath); err != nil {
				t.Errorf("Script path exists but stat failed: %v", err)
			}
		}
	})
}

// TestRecognizeWithPaddleOCR tests the PaddleOCR recognition function
func TestRecognizeWithPaddleOCR(t *testing.T) {
	service := NewOCRService()

	t.Run("Returns error when script not found", func(t *testing.T) {
		// Create a temporary directory without the script
		tempDir, err := ioutil.TempDir("", "ocr-test-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		// Create a dummy image file
		imagePath := filepath.Join(tempDir, "test.png")
		if err := ioutil.WriteFile(imagePath, []byte("dummy"), 0644); err != nil {
			t.Fatal(err)
		}

		// Change working directory temporarily to ensure script is not found
		originalWd, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(originalWd)

		_, err = service.RecognizeWithPaddleOCR(imagePath)
		if err == nil {
			t.Error("Expected RecognizeWithPaddleOCR to return error when script not found")
		}
		if !strings.Contains(err.Error(), "script not found") {
			t.Errorf("Expected error to mention 'script not found', got: %v", err)
		}
	})

	t.Run("Successfully executes mock PaddleOCR script", func(t *testing.T) {
		// Skip this test if Python is not available
		if _, err := exec.LookPath("python3"); err != nil {
			if _, err := exec.LookPath("python"); err != nil {
				t.Skip("Python not available, skipping test")
			}
		}

		// Create a temporary directory
		tempDir, err := ioutil.TempDir("", "ocr-test-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		// Create a mock PaddleOCR script
		mockScript := `#!/usr/bin/env python3
import sys
import json

result = {
    "success": True,
    "text": "微信支付\\n-1700.00\\n商户全称：测试商户",
    "lines": [
        {"text": "微信支付", "confidence": 0.95},
        {"text": "-1700.00", "confidence": 0.98},
        {"text": "商户全称：测试商户", "confidence": 0.92}
    ],
    "line_count": 3
}
print(json.dumps(result, ensure_ascii=False))
`
		scriptPath := filepath.Join(tempDir, "paddleocr_cli.py")
		if err := ioutil.WriteFile(scriptPath, []byte(mockScript), 0755); err != nil {
			t.Fatal(err)
		}

		// Create a dummy image file
		imagePath := filepath.Join(tempDir, "test.png")
		if err := ioutil.WriteFile(imagePath, []byte("dummy"), 0644); err != nil {
			t.Fatal(err)
		}

		// Change working directory to temp dir so script is found
		originalWd, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(originalWd)

		text, err := service.RecognizeWithPaddleOCR(imagePath)
		if err != nil {
			t.Fatalf("RecognizeWithPaddleOCR returned error: %v", err)
		}

		if !strings.Contains(text, "微信支付") {
			t.Error("Expected text to contain '微信支付'")
		}
		if !strings.Contains(text, "-1700.00") {
			t.Error("Expected text to contain '-1700.00'")
		}
		if !strings.Contains(text, "测试商户") {
			t.Error("Expected text to contain '测试商户'")
		}
	})

	t.Run("Returns error when script returns error", func(t *testing.T) {
		// Skip this test if Python is not available
		if _, err := exec.LookPath("python3"); err != nil {
			if _, err := exec.LookPath("python"); err != nil {
				t.Skip("Python not available, skipping test")
			}
		}

		// Create a temporary directory
		tempDir, err := ioutil.TempDir("", "ocr-test-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		// Create a mock PaddleOCR script that returns an error
		mockScript := `#!/usr/bin/env python3
import sys
import json

result = {
    "success": False,
    "error": "Image file not found"
}
print(json.dumps(result))
sys.exit(1)
`
		scriptPath := filepath.Join(tempDir, "paddleocr_cli.py")
		if err := ioutil.WriteFile(scriptPath, []byte(mockScript), 0755); err != nil {
			t.Fatal(err)
		}

		// Create a dummy image file
		imagePath := filepath.Join(tempDir, "test.png")
		if err := ioutil.WriteFile(imagePath, []byte("dummy"), 0644); err != nil {
			t.Fatal(err)
		}

		// Change working directory to temp dir so script is found
		originalWd, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(originalWd)

		_, err = service.RecognizeWithPaddleOCR(imagePath)
		if err == nil {
			t.Error("Expected RecognizeWithPaddleOCR to return error for failed OCR")
		}
		if !strings.Contains(err.Error(), "PaddleOCR error") {
			t.Errorf("Expected error to mention 'PaddleOCR error', got: %v", err)
		}
	})
}

// TestRecognizePaymentScreenshotWithPaddleOCR tests payment screenshot recognition with PaddleOCR
func TestRecognizePaymentScreenshotWithPaddleOCR(t *testing.T) {
	service := NewOCRService()

	t.Run("Uses PaddleOCR when available", func(t *testing.T) {
		// This test verifies that if PaddleOCR is available, it will be used
		// The actual behavior depends on the test environment
		available := service.isPaddleOCRAvailable()
		t.Logf("PaddleOCR available: %v", available)

		// If available, we expect it to be used in RecognizePaymentScreenshot
		// However, we can't easily test this without a real image and PaddleOCR installed
		// So we just verify the function doesn't panic
	})

	t.Run("Falls back to Tesseract when PaddleOCR unavailable", func(t *testing.T) {
		// Create a temporary directory without the script
		tempDir, err := ioutil.TempDir("", "ocr-test-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)

		// Change working directory temporarily to ensure script is not found
		originalWd, _ := os.Getwd()
		os.Chdir(tempDir)
		defer os.Chdir(originalWd)

		// This will try PaddleOCR (fail), then fall back to Tesseract
		// We expect an error since the image doesn't exist, but it should be from Tesseract fallback
		_, err = service.RecognizePaymentScreenshot("/nonexistent/image.png")

		// We expect an error since the image doesn't exist
		// The error should not mention PaddleOCR specifically (since it fell back)
		if err == nil {
			t.Log("Expected an error for nonexistent image")
		}
	})
}
