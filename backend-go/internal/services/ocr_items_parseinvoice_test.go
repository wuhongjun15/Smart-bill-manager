package services

import "testing"

func TestParseInvoiceDataWithMeta_Items_ParseAndPeelUnitSpec(t *testing.T) {
	svc := &OCRService{}
	text := `
【发票信息】
发票号码： 25312000000427429429
开票日期： 2025年12月23日
电子发票（普通发票）
【购买方】
项目名称 规格型号 单位 数量
【明细】
*餐饮服务*餐饮服务项项1
税率/征收率 6%
合计 ￥ 2483.02 ￥ 148.98
价税合计（小写） ￥ 2632.00
`
	data, err := svc.ParseInvoiceDataWithMeta(text, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Items) != 1 {
		t.Fatalf("expected 1 item, got %d: %+v", len(data.Items), data.Items)
	}
	it := data.Items[0]
	if it.Unit != "项" || it.Spec != "项" || it.Quantity == nil || *it.Quantity != 1 {
		t.Fatalf("unexpected item parsed: %+v", it)
	}
}
