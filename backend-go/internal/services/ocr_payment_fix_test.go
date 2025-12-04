package services

import (
	"strings"
	"testing"
)

// TestParsePaymentScreenshot_ProblemStatementCase tests the exact case from the problem statement
func TestParsePaymentScreenshot_ProblemStatementCase(t *testing.T) {
	service := NewOCRService()

	// Exact OCR text from the problem statement
	sampleText := `14:59 回 怨 5501l| @
主 全 部 账 单
当 心
A
海 烟 烟 行

当 前 状 态 支 付 成 功

支 付 时 间 2025 年 10 月 23 日 14:59:46

商 品 海 烟 烟 行 ( 上 海 郡 徕 实 业 有 限 公 司
910360)

商 户 全 称 上 海 郡 徕 实 业 有 限 公 司

收 单 机 构 通 联 支 付 网 络 服 务 股 份 有 限 公 司
由 中 国 银 联 股 份 有 限 公 司 提 供 收 款 清 算
服 务

支 付 方 式 招 商 银 行 信 用 卡 (2506)
由 网 联 清 算 有 限 公 司 提 供 付 款 清 算 服 务

交 易 单 号 4200002966202510230090527049

商 户 单 号 251023116574060365`

	data, err := service.ParsePaymentScreenshot(sampleText)
	if err != nil {
		t.Fatalf("ParsePaymentScreenshot returned error: %v", err)
	}

	// Test merchant extraction - should extract "海烟烟行" from "商品" field
	if data.Merchant == nil {
		t.Error("Merchant is nil - should extract merchant name")
	} else {
		t.Logf("Extracted merchant: '%s'", *data.Merchant)
		// Should be "海烟烟行" (prioritized) or "上海郡徕实业有限公司"
		validMerchants := []string{"海烟烟行", "上海郡徕实业有限公司"}
		found := false
		for _, valid := range validMerchants {
			if *data.Merchant == valid {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected merchant to be one of %v, got '%s'", validMerchants, *data.Merchant)
		}
	}

	// Test transaction time extraction - CRITICAL FIX
	// Expected: "2025-10-23 14:59:46"
	if data.TransactionTime == nil {
		t.Error("TransactionTime is nil - should extract '2025-10-23 14:59:46'")
	} else {
		t.Logf("Extracted time: '%s'", *data.TransactionTime)
		expectedTime := "2025-10-23 14:59:46"
		if *data.TransactionTime != expectedTime {
			t.Errorf("Expected TransactionTime '%s', got '%s'", expectedTime, *data.TransactionTime)
		}
	}

	// Test order number extraction
	if data.OrderNumber == nil {
		t.Error("OrderNumber is nil - should extract order number")
	} else {
		t.Logf("Extracted order number: '%s'", *data.OrderNumber)
		expectedOrderNum := "4200002966202510230090527049"
		if *data.OrderNumber != expectedOrderNum {
			t.Errorf("Expected OrderNumber '%s', got '%s'", expectedOrderNum, *data.OrderNumber)
		}
	}

	// Test payment method extraction
	if data.PaymentMethod == nil {
		t.Error("PaymentMethod is nil")
	} else {
		t.Logf("Extracted payment method: '%s'", *data.PaymentMethod)
	}
}

// TestRemoveChineseSpaces_PreserveTimeSpace tests the fix for preserving space after 日
func TestRemoveChineseSpaces_PreserveTimeSpace(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Date with spaces - preserve space before time",
			input:    "2025 年 10 月 23 日 14:59:46",
			expected: "2025年10月23日 14:59:46",
		},
		{
			name:     "Payment time text from problem statement",
			input:    "支 付 时 间 2025 年 10 月 23 日 14:59:46",
			expected: "支付时间2025年10月23日 14:59:46",
		},
		{
			name:     "Date only - remove all spaces",
			input:    "2025 年 10 月 23 日",
			expected: "2025年10月23日",
		},
		{
			name:     "Time with different format",
			input:    "支 付 时 间 2025 年 10 月 23 日 09:30:15",
			expected: "支付时间2025年10月23日 09:30:15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeChineseSpaces(tt.input)
			if result != tt.expected {
				t.Errorf("removeChineseSpaces(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConvertChineseDateToISO_BothFormats tests the improved convertChineseDateToISO
func TestConvertChineseDateToISO_BothFormats(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Date and time with space",
			input:    "2025年10月23日 14:59:46",
			expected: "2025-10-23 14:59:46",
		},
		{
			name:     "Date and time without space",
			input:    "2025年10月23日14:59:46",
			expected: "2025-10-23 14:59:46",
		},
		{
			name:     "Date only",
			input:    "2025年10月23日",
			expected: "2025-10-23",
		},
		{
			name:     "Single digit month and day with time",
			input:    "2025年1月5日 9:30:46",
			expected: "2025-1-5 9:30:46",
		},
		{
			name:     "Single digit month and day without space",
			input:    "2025年1月5日9:30:46",
			expected: "2025-1-5 9:30:46",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertChineseDateToISO(tt.input)
			if result != tt.expected {
				t.Errorf("convertChineseDateToISO(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParsePaymentScreenshot_WithNegativeAmount tests parsing with negative amount
func TestParsePaymentScreenshot_WithNegativeAmount(t *testing.T) {
	service := NewOCRService()

	// Test with negative amount (as mentioned in problem statement)
	sampleText := `支 付 成 功
-1700.00
支 付 时 间 2025 年 10 月 23 日 14:59:46
商 品 海 烟 烟 行
交 易 单 号 4200002966202510230090527049`

	data, err := service.ParsePaymentScreenshot(sampleText)
	if err != nil {
		t.Fatalf("ParsePaymentScreenshot returned error: %v", err)
	}

	// Test amount extraction
	if data.Amount == nil {
		t.Error("Amount is nil - should extract 1700.00 from -1700.00")
	} else {
		expectedAmount := 1700.00
		if *data.Amount != expectedAmount {
			t.Errorf("Expected Amount %.2f, got %.2f", expectedAmount, *data.Amount)
		}
	}

	// Test time extraction
	if data.TransactionTime == nil {
		t.Error("TransactionTime is nil")
	} else {
		expectedTime := "2025-10-23 14:59:46"
		if *data.TransactionTime != expectedTime {
			t.Errorf("Expected TransactionTime '%s', got '%s'", expectedTime, *data.TransactionTime)
		}
	}
}

// Helper function to check if string contains any of the substrings
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
