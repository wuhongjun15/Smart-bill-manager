package services

import "testing"

func TestExtractCompanyNameNearTaxID_PicksLongestNestedMatch(t *testing.T) {
	text := "统一社会信用代码/纳税人识别号: 北京易行出行旅游有限公司91110108735575307R单价65.42金额65.42税率/征收率6% 税额3.92"
	got := extractCompanyNameNearTaxID(text)
	if got != "北京易行出行旅游有限公司" {
		t.Fatalf("company name mismatch: got=%q", got)
	}
}

