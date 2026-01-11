package services

import (
	"strings"
	"testing"
)

func TestFixInvoiceZonesForPretty_MergesFakePasswordZoneIntoBuyer(t *testing.T) {
	// Simulate a template without password area: fixed-region split still emits "【密码区】",
	// and buyer/table text leaks into it. The pretty fixer should merge/remove it.
	in := []string{
		"【第1页-分区】",
		"【发票信息】",
		"发票号码：26117000000093487418",
		"【购买方】",
		"购买方信息名称：个人",
		"项目名称 规格型号 单位 数量",
		"【密码区】",
		"购买方信息名称：个人",
		"地址、电话：北京",
		"【明细】",
		"*旅游服务*代订车服务费 个 1",
	}

	out := fixInvoiceZonesForPretty(in, nil)
	pretty := strings.Join(out, "\n")
	if strings.Contains(pretty, "【密码区】") {
		t.Fatalf("expected fake 【密码区】 removed/merged, got:\n%s", pretty)
	}
	if !strings.Contains(pretty, "【购买方】") || !strings.Contains(pretty, "购买方信息名称：个人") {
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

func TestFixInvoiceZonesForPretty_DistributesMergedLinesToSellerAndDetail(t *testing.T) {
	in := []string{
		"【第1页-分区】",
		"【发票信息】",
		"发票号码：26117000000093487418",
		"【购买方】",
		"购买方信息名称：个人",
		"【密码区】",
		"项目名称 规格型号 单位 数量",
		"统一社会信用代码/纳税人识别号: 北京易行出行旅游有限公司 91110108735575307R",
		"单价 65.42 金额 65.42 税率/征收率 6% 税额 3.92",
		"【明细】",
		"*旅游服务*代订车服务费 个 1",
		"【销售方】",
		"备注",
	}

	data := &InvoiceExtractedData{
		BuyerName:  ptrString("个人"),
		SellerName: ptrString("北京易行出行旅游有限公司"),
	}

	out := fixInvoiceZonesForPretty(in, data)
	pretty := strings.Join(out, "\n")
	if strings.Contains(pretty, "【密码区】") {
		t.Fatalf("expected fake 【密码区】 removed/merged, got:\n%s", pretty)
	}

	sellerIdx := strings.Index(pretty, "【销售方】")
	if sellerIdx == -1 || !strings.Contains(pretty[sellerIdx:], "北京易行出行旅游有限公司") {
		t.Fatalf("expected seller zone to include company line, got:\n%s", pretty)
	}
	if strings.Contains(pretty[sellerIdx:], "税率/征收率") {
		t.Fatalf("expected tax columns to be moved to 【明细】, got seller zone:\n%s", pretty[sellerIdx:])
	}

	detailIdx := strings.Index(pretty, "【明细】")
	if detailIdx == -1 || !strings.Contains(pretty[detailIdx:], "税率/征收率") {
		t.Fatalf("expected detail zone to include tax columns, got:\n%s", pretty)
	}
}

