package medqlab

import "testing"

func TestResolvePanelTariffPanelWins(t *testing.T) {
	panel := panelTariff{TotalByr: 300000, BagianRS: 300000}
	items := []templateTariff{{BiayaItem: 55000, BagianRS: 55000}}
	got := resolvePanelTariff(panel, items)
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
	got := resolvePanelTariff(panel, items)
	if got.TotalByr != 55000 || got.BagianRS != 55000 || got.BHP != 100 {
		t.Fatalf("want rollup 55000/55000/100, got %+v", got)
	}
}

func TestResolvePanelTariffAllZero(t *testing.T) {
	panel := panelTariff{}
	items := []templateTariff{{BiayaItem: 0}, {BiayaItem: 0}}
	got := resolvePanelTariff(panel, items)
	if got.TotalByr != 0 {
		t.Fatalf("want 0, got %+v", got)
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
