package medqlab

import "testing"

func TestResolvePanelTariffPanelWins(t *testing.T) {
	panel := panelTariff{TotalByr: 300000, BagianRS: 300000}
	items := []templateTariff{{BiayaItem: 55000, BagianRS: 55000}}
	got, mode := resolvePanelTariff(panel, items)
	if mode != tarifModeTindakan {
		t.Fatalf("want mode=tindakan, got %s", mode)
	}
	if got.TotalByr != 300000 || got.BagianRS != 300000 {
		t.Fatalf("panel should win: %+v", got)
	}
}

func TestResolvePanelTariffTemplateKeepsPanelZero(t *testing.T) {
	// Khanza native: panel total_byr=0, charge on items — do not roll up onto periksa_lab.
	panel := panelTariff{}
	items := []templateTariff{
		{BiayaItem: 40000, BagianRS: 40000},
		{BiayaItem: 40000, BagianRS: 40000},
		{BiayaItem: 40000, BagianRS: 40000},
	}
	got, mode := resolvePanelTariff(panel, items)
	if mode != tarifModeTemplate {
		t.Fatalf("want mode=template, got %s", mode)
	}
	if got.TotalByr != 0 || got.BagianRS != 0 {
		t.Fatalf("panel must stay 0 in template mode, got %+v", got)
	}
}

func TestResolvePanelTariffAllZero(t *testing.T) {
	panel := panelTariff{}
	items := []templateTariff{{BiayaItem: 0}, {BiayaItem: 0}}
	got, mode := resolvePanelTariff(panel, items)
	if mode != tarifModeNone {
		t.Fatalf("want mode=none, got %s", mode)
	}
	if got.TotalByr != 0 {
		t.Fatalf("want 0, got %+v", got)
	}
}

func TestDetailTariffForModeTindakanZeros(t *testing.T) {
	tpl := templateTariff{
		IDTemplate: 10, KdJenisPrw: "002-A-K3",
		BagianRS: 40000, BHP: 1, BagianPerujuk: 2, BagianDokter: 3,
		BagianLaborat: 4, KSO: 5, Menejemen: 6, BiayaItem: 40000,
	}
	got := detailTariffForMode(tarifModeTindakan, tpl)
	if got.IDTemplate != 10 || got.KdJenisPrw != "002-A-K3" {
		t.Fatalf("ids should be kept: %+v", got)
	}
	if got.BiayaItem != 0 || got.BagianRS != 0 || got.BHP != 0 ||
		got.BagianPerujuk != 0 || got.BagianDokter != 0 || got.BagianLaborat != 0 ||
		got.KSO != 0 || got.Menejemen != 0 {
		t.Fatalf("tindakan detail tariffs must be zero: %+v", got)
	}
}

func TestDetailTariffForModeTemplateKeeps(t *testing.T) {
	tpl := templateTariff{
		IDTemplate: 11, KdJenisPrw: "002-A-K3",
		BagianRS: 40000, BiayaItem: 40000,
	}
	got := detailTariffForMode(tarifModeTemplate, tpl)
	if got.BiayaItem != 40000 || got.BagianRS != 40000 || got.IDTemplate != 11 {
		t.Fatalf("template mode should keep item tariff: %+v", got)
	}
}

func TestDetailTariffForModeNoneZeros(t *testing.T) {
	tpl := templateTariff{IDTemplate: 12, BiayaItem: 99}
	got := detailTariffForMode(tarifModeNone, tpl)
	if got.BiayaItem != 0 || got.IDTemplate != 12 {
		t.Fatalf("none mode: %+v", got)
	}
}

func TestSumTemplateTariffsMapsFields(t *testing.T) {
	got := sumTemplateTariffs([]templateTariff{
		{
			BagianRS: 10, BHP: 20, BagianPerujuk: 30, BagianDokter: 40,
			BagianLaborat: 50, KSO: 60, Menejemen: 70, BiayaItem: 100,
		},
		{
			BagianRS: 1, BHP: 2, BagianPerujuk: 3, BagianDokter: 4,
			BagianLaborat: 5, KSO: 6, Menejemen: 7, BiayaItem: 8,
		},
	})
	if got.TotalByr != 108 || got.BagianRS != 11 || got.BHP != 22 ||
		got.TarifPerujuk != 33 || got.TarifTindakanDokter != 44 ||
		got.TarifTindakanPetugas != 55 || got.KSO != 66 || got.Menejemen != 77 {
		t.Fatalf("sum fields: %+v", got)
	}
}

func TestFeeOnlyOrderedTemplatesElektrolit(t *testing.T) {
	// Fee header ELEKTROLIT 300k not in hasil; ion leaves written with biaya 0.
	ordered := []templateTariff{
		{IDTemplate: 5345, BiayaItem: 300000, BagianRS: 300000},
		{IDTemplate: 5346, BiayaItem: 0},
		{IDTemplate: 5347, BiayaItem: 0},
		{IDTemplate: 5348, BiayaItem: 0},
	}
	written := map[int]bool{5346: true, 5347: true, 5348: true}
	got := feeOnlyOrderedTemplates(ordered, written)
	if len(got) != 1 || got[0].IDTemplate != 5345 || got[0].BiayaItem != 300000 {
		t.Fatalf("want only ELEKTROLIT fee: %+v", got)
	}
}

func TestFeeOnlyOrderedTemplatesKimiaEmpty(t *testing.T) {
	// All priced items already written as hasil — no fee-only rows.
	ordered := []templateTariff{
		{IDTemplate: 1, BiayaItem: 40000},
		{IDTemplate: 2, BiayaItem: 40000},
		{IDTemplate: 3, BiayaItem: 40000},
	}
	written := map[int]bool{1: true, 2: true, 3: true}
	got := feeOnlyOrderedTemplates(ordered, written)
	if len(got) != 0 {
		t.Fatalf("want no fee-only, got %+v", got)
	}
}

func TestFeeOnlySkipsUnpricedAndAlreadyWritten(t *testing.T) {
	ordered := []templateTariff{
		{IDTemplate: 10, BiayaItem: 50000},
		{IDTemplate: 11, BiayaItem: 0},
		{IDTemplate: 12, BiayaItem: 25000},
	}
	written := map[int]bool{10: true}
	got := feeOnlyOrderedTemplates(ordered, written)
	if len(got) != 1 || got[0].IDTemplate != 12 {
		t.Fatalf("want only id 12: %+v", got)
	}
}

func TestIsPricedTemplate(t *testing.T) {
	if isPricedTemplate(templateTariff{}) {
		t.Fatal("zero should be unpriced")
	}
	if !isPricedTemplate(templateTariff{BiayaItem: 1}) {
		t.Fatal("biaya_item")
	}
	if !isPricedTemplate(templateTariff{BagianRS: 1}) {
		t.Fatal("bagian_rs alone counts")
	}
}
