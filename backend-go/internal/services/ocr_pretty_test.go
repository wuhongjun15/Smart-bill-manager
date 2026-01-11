package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeInvoiceTextForPretty_KeepPasswordZone_WhenItLooksLegit(t *testing.T) {
	// This sample has a real "【密码区】" section that contains seller/tax-related lines (not encrypted),
	// and we should not "fix" it by moving it into 买方/销售方/明细 in the pretty output.
	path := filepath.Join("testdata", "regression", "invoices", "20260110_26117000000093487418_yixing_seller_full_pymupdf_zones.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}

	var s regressionSample
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("unmarshal sample: %v", err)
	}

	svc := NewOCRService()
	data, err := svc.ParseInvoiceData(s.RawText)
	if err != nil {
		t.Fatalf("parse invoice: %v", err)
	}

	clean := normalizeInvoiceTextForPretty(s.RawText, data)
	if !strings.Contains(clean, "【密码区】") {
		t.Fatalf("expected pretty text to keep 【密码区】, got:\n%s", clean)
	}
}

func TestNormalizeInvoiceTextForPretty_MergePasswordZone_WhenBuyerLeaksIntoIt(t *testing.T) {
	// Synthetic "no password area" layout: fixed-region split emits "【密码区】" but the content is clearly buyer/table info.
	raw := strings.Join([]string{
		"【第1页-分区】",
		"【发票信息】",
		"发票号码： 123",
		"开票日期： 2026年01月01日",
		"【购买方】",
		"购买方信息名称： 张三",
		"项目名称 规格型号 单位 数量",
		"【密码区】",
		"购买方信息名称： 张三",
		"地址、电话： 北京",
		"【明细】",
		"*服务*测试 个 1",
		"【销售方】",
		"销售方信息名称： 测试有限公司",
	}, "\n")

	buyer := "张三"
	seller := "测试有限公司"
	data := &InvoiceExtractedData{
		BuyerName:  &buyer,
		SellerName: &seller,
	}

	clean := normalizeInvoiceTextForPretty(raw, data)
	if strings.Contains(clean, "【密码区】") {
		t.Fatalf("expected pretty text to merge/remove fake 【密码区】, got:\n%s", clean)
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(clean, " ", ""), "\t", "")
	if !strings.Contains(compact, "【购买方】") || !strings.Contains(compact, "购买方信息名称：张三") {
		t.Fatalf("expected buyer info to be present in pretty text, got:\n%s", clean)
	}
}
