package services

import (
	"strings"
	"testing"
)

func TestNormalizeInvoiceTextForPretty_MergesNonPasswordPasswordZoneIntoBuyer(t *testing.T) {
	// This test targets the zoned pretty-text fix (not the general spacing normalizer).
	in := []string{
		"【第1页-分区】",
		"【发票信息】",
		"发票号码： 26117000000093487418",
		"【购买方】",
		"购买方信息名称： 个人",
		"【密码区】",
		"统一社会信用代码/纳税人识别号: 北京易行出行旅游有限公司91110108735575307R",
		"单价65.42金额65.42税率/征收率6% 税额3.92",
		"【明细】",
		"*旅游服务*代订车服务费 个 1",
	}
	block := strings.Join(in, "\n")
	if !strings.Contains(block, "纳税人识别号") || !strings.Contains(block, "统一社会信用代码") {
		t.Fatalf("test setup invalid: block should contain buyer/seller markers, got:\n%s", block)
	}
	if in[5] != "【密码区】" {
		t.Fatalf("test setup invalid: expected header literal match, got=%q", in[5])
	}
	out := fixInvoiceZonesForPretty(in, nil)
	pretty := strings.Join(out, "\n")
	if strings.Contains(pretty, "【密码区】") {
		t.Fatalf("expected password zone removed/merged, got:\n%s", pretty)
	}
	if !strings.Contains(pretty, "【购买方】") || !strings.Contains(pretty, "北京易行出行旅游有限公司") {
		t.Fatalf("expected buyer zone to include merged content, got:\n%s", pretty)
	}
}

func TestNormalizeInvoiceTextForPretty_KeepRealPasswordZone(t *testing.T) {
	raw := strings.Join([]string{
		"【第1页-分区】",
		"【购买方】",
		"名称: 武亚峰",
		"【密码区】",
		"密 码 区 200.00 *14<<*>07/6>27/*88780<>*>45",
		"【明细】",
		"*电信服务*话费充值 元 1",
	}, "\n")

	pretty := normalizeInvoiceTextForPretty(raw, nil)
	if !strings.Contains(pretty, "【密码区】") {
		t.Fatalf("expected password zone preserved, got:\n%s", pretty)
	}
}

func TestFixInvoiceZonesForPretty_PlaceNonPasswordPasswordZoneIntoSellerWhenKnown(t *testing.T) {
	in := []string{
		"【第1页-分区】",
		"【发票信息】",
		"发票号码： 26117000000093487418",
		"【购买方】",
		"购 买 方 信 息 名称： 统一社会信用代码/纳税人识别号: 个人 销 售 方 信 息 名称：",
		"【密码区】",
		"统一社会信用代码/纳税人识别号: 北京易行出行旅游有限公司 91110108735575307R",
		"单价 65.42 金额 65.42 税率/征收率 6% 税额 3.92",
		"【明细】",
		"*旅游服务*代订车服务费 个 1",
		"【销售方】",
		"备 注",
	}

	data := &InvoiceExtractedData{
		BuyerName:  ptrString("个人"),
		SellerName: ptrString("北京易行出行旅游有限公司"),
	}

	out := fixInvoiceZonesForPretty(in, data)
	pretty := strings.Join(out, "\n")
	if strings.Contains(pretty, "【密码区】") {
		t.Fatalf("expected password zone removed/merged, got:\n%s", pretty)
	}

	// Seller should contain the company+taxid line.
	sellerIdx := strings.Index(pretty, "【销售方】")
	if sellerIdx == -1 {
		t.Fatalf("expected seller zone, got:\n%s", pretty)
	}
	if !strings.Contains(pretty[sellerIdx:], "北京易行出行旅游有限公司") {
		t.Fatalf("expected seller zone to include company line, got:\n%s", pretty)
	}
	if strings.Contains(pretty[sellerIdx:], "税率/征收率") {
		t.Fatalf("expected tax columns to be moved to 明细, got seller zone:\n%s", pretty[sellerIdx:])
	}

	// 明细 should contain the tax/amount column line.
	detailIdx := strings.Index(pretty, "【明细】")
	if detailIdx == -1 {
		t.Fatalf("expected detail zone, got:\n%s", pretty)
	}
	if !strings.Contains(pretty[detailIdx:], "税率/征收率") {
		t.Fatalf("expected detail zone to include tax columns, got:\n%s", pretty[detailIdx:])
	}

	// Buyer zone should not contain the seller tax-id line anymore.
	buyerIdx := strings.Index(pretty, "【购买方】")
	if buyerIdx == -1 {
		t.Fatalf("expected buyer zone, got:\n%s", pretty)
	}
	afterBuyer := pretty[buyerIdx:]
	if strings.Contains(afterBuyer, "91110108735575307R") && (sellerIdx == -1 || buyerIdx < sellerIdx) {
		// Ensure the tax-id is not still in buyer block before seller header.
		if sellerIdx != -1 {
			buyerBlock := pretty[buyerIdx:sellerIdx]
			if strings.Contains(buyerBlock, "91110108735575307R") {
				t.Fatalf("expected buyer zone not to include seller tax-id line, got:\n%s", buyerBlock)
			}
		}
	}
}
