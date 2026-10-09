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

func TestResolvePanelTariffRollupFromItems(t *testing.T) {
	panel := panelTariff{} // total_byr = 0
	items := []templateTariff{
		{BiayaItem: 55000, BagianRS: 55000},
		{BiayaItem: 0, BagianRS: 0, BHP: 100}, // still summed if any cost fields set
	}
	got, mode := resolvePanelTariff(panel, items)
	if mode != tarifModeTemplate {
		t.Fatalf("want mode=template, got %s", mode)
	}
	if got.TotalByr != 55000 || got.BagianRS != 55000 || got.BHP != 100 {
		t.Fatalf("want rollup 55000/55000/100, got %+v", got)
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

func TestDetailTariffForModeTemplateZeros(t *testing.T) {
	// Panel already holds rollup; detail must be 0 or Khanza UI double-counts.
	tpl := templateTariff{
		IDTemplate: 11, KdJenisPrw: "082-A-K3",
		BagianRS: 55000, BiayaItem: 55000,
	}
	got := detailTariffForMode(tarifModeTemplate, tpl)
	if got.BiayaItem != 0 || got.BagianRS != 0 || got.IDTemplate != 11 {
		t.Fatalf("template mode detail tariffs must be zero: %+v", got)
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
