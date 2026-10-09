package medqlab

import (
	"testing"
	"time"
)

func TestFormatNoJurnal(t *testing.T) {
	day := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	if got := formatNoJurnal(day, ""); got != "JR20261008000001" {
		t.Fatalf("empty last: got %s", got)
	}
	if got := formatNoJurnal(day, "JR20261008000007"); got != "JR20261008000008" {
		t.Fatalf("next seq: got %s", got)
	}
	if got := formatNoJurnal(day, "JR20261007000099"); got != "JR20261008000001" {
		t.Fatalf("other day: got %s", got)
	}
}

func TestBuildJournalLinesPairsAndSkipZero(t *testing.T) {
	coa := labCOA{
		SuspenPiutang:    "A1",
		Laborat:          "A2",
		BebanJasaDokter:  "B1",
		UtangJasaDokter:  "B2",
		BebanJasaPetugas: "C1",
		UtangJasaPetugas: "C2",
		HPPPersediaan:    "D1",
		PersediaanBHP:    "D2",
		BebanKSO:         "E1",
		UtangKSO:         "E2",
		BebanJasaSarana:  "F1",
		UtangJasaSarana:  "F2",
		BebanJasaPerujuk: "G1",
		UtangJasaPerujuk: "G2",
		BebanMenejemen:   "H1",
		UtangMenejemen:   "H2",
	}
	lines := buildJournalLines(coa, journalTotals{
		Pendapatan: 100,
		JasaDokter: 10,
		BHP:        0, // skipped
	})
	if len(lines) != 4 {
		t.Fatalf("want 4 lines (2 pairs), got %d", len(lines))
	}
	debet, kredit, ok := journalBalanceOK(lines)
	if !ok || debet != 110 || kredit != 110 {
		t.Fatalf("balance debet=%.2f kredit=%.2f ok=%v", debet, kredit, ok)
	}
}

func TestBuildJournalLinesSkipEmptyAccount(t *testing.T) {
	coa := labCOA{SuspenPiutang: "A1"} // Laborat empty → pair skipped
	lines := buildJournalLines(coa, journalTotals{Pendapatan: 50})
	if len(lines) != 0 {
		t.Fatalf("want 0 lines when kredit kd_rek empty, got %d", len(lines))
	}
}

func TestJournalTotalsAddPanelOnly(t *testing.T) {
	// writeSIMRS journals resolved panel tariff only (no addDetail) to avoid double-count.
	var tot journalTotals
	tot.addPanel(panelTariff{
		BagianRS: 1, BHP: 2, TarifPerujuk: 3, TarifTindakanDokter: 4,
		TarifTindakanPetugas: 5, KSO: 6, Menejemen: 7, TotalByr: 100,
	})
	if tot.Pendapatan != 100 || tot.JasaSarana != 1 || tot.BHP != 2 {
		t.Fatalf("panel totals: %+v", tot)
	}
	if tot.isZero() {
		t.Fatal("expected non-zero")
	}
}

func TestJournalKeterangan(t *testing.T) {
	if got := journalKeterangan("ralan", "NIP1"); got != "PEMERIKSAAN LABORAT RAWAT JALAN, DIPOSTING OLEH NIP1" {
		t.Fatalf("ralan: %s", got)
	}
	if got := journalKeterangan("Ranap", "NIP2"); got != "PEMERIKSAAN LABORAT RAWAT INAP, DIPOSTING OLEH NIP2" {
		t.Fatalf("ranap: %s", got)
	}
}
