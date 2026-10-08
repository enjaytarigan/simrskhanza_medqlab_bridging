package medqlab

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

// journalTotals accumulates lab tariff amounts for accounting (Khanza simpanlab).
type journalTotals struct {
	Pendapatan   float64
	JasaDokter   float64
	JasaPetugas  float64
	BHP          float64
	KSO          float64
	JasaSarana   float64
	JasaPerujuk  float64
	Menejemen    float64
}

func (t *journalTotals) addPanel(p panelTariff) {
	t.JasaSarana += p.BagianRS
	t.BHP += p.BHP
	t.JasaPerujuk += p.TarifPerujuk
	t.JasaDokter += p.TarifTindakanDokter
	t.JasaPetugas += p.TarifTindakanPetugas
	t.KSO += p.KSO
	t.Menejemen += p.Menejemen
	t.Pendapatan += p.TotalByr
}

func (t *journalTotals) addDetail(d templateTariff) {
	t.JasaSarana += d.BagianRS
	t.BHP += d.BHP
	t.JasaPerujuk += d.BagianPerujuk
	t.JasaDokter += d.BagianDokter
	t.JasaPetugas += d.BagianLaborat
	t.KSO += d.KSO
	t.Menejemen += d.Menejemen
	t.Pendapatan += d.BiayaItem
}

func (t journalTotals) isZero() bool {
	return t.Pendapatan == 0 && t.JasaDokter == 0 && t.JasaPetugas == 0 &&
		t.BHP == 0 && t.KSO == 0 && t.JasaSarana == 0 && t.JasaPerujuk == 0 && t.Menejemen == 0
}

// labCOA holds Chart of Accounts codes from set_akun_ralan / set_akun_ranap.
type labCOA struct {
	SuspenPiutang     string
	Laborat           string
	BebanJasaDokter   string
	UtangJasaDokter   string
	BebanJasaPetugas  string
	UtangJasaPetugas  string
	BebanKSO          string
	UtangKSO          string
	HPPPersediaan     string
	PersediaanBHP     string
	BebanJasaSarana   string
	UtangJasaSarana   string
	BebanJasaPerujuk  string
	UtangJasaPerujuk  string
	BebanMenejemen    string
	UtangMenejemen    string
}

type journalLine struct {
	KdRek  string
	Debet  float64
	Kredit float64
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// buildJournalLines maps totals to debet/kredit pairs (Khanza / AdamLabs).
// Zero amounts and empty kd_rek are skipped.
func buildJournalLines(coa labCOA, t journalTotals) []journalLine {
	type pair struct {
		amount float64
		debet  string
		kredit string
	}
	pairs := []pair{
		{t.Pendapatan, coa.SuspenPiutang, coa.Laborat},
		{t.JasaDokter, coa.BebanJasaDokter, coa.UtangJasaDokter},
		{t.JasaPetugas, coa.BebanJasaPetugas, coa.UtangJasaPetugas},
		{t.BHP, coa.HPPPersediaan, coa.PersediaanBHP},
		{t.KSO, coa.BebanKSO, coa.UtangKSO},
		{t.JasaSarana, coa.BebanJasaSarana, coa.UtangJasaSarana},
		{t.JasaPerujuk, coa.BebanJasaPerujuk, coa.UtangJasaPerujuk},
		{t.Menejemen, coa.BebanMenejemen, coa.UtangMenejemen},
	}
	var lines []journalLine
	for _, p := range pairs {
		amt := round2(p.amount)
		if amt <= 0 {
			continue
		}
		debet := strings.TrimSpace(p.debet)
		kredit := strings.TrimSpace(p.kredit)
		if debet == "" || kredit == "" {
			continue
		}
		lines = append(lines,
			journalLine{KdRek: debet, Debet: amt, Kredit: 0},
			journalLine{KdRek: kredit, Debet: 0, Kredit: amt},
		)
	}
	return lines
}

func journalBalanceOK(lines []journalLine) (debet, kredit float64, ok bool) {
	for _, l := range lines {
		debet += l.Debet
		kredit += l.Kredit
	}
	debet, kredit = round2(debet), round2(kredit)
	return debet, kredit, debet == kredit
}

// formatNoJurnal builds JR{YYYYMMDD}{6-digit seq} from the latest same-day number.
func formatNoJurnal(day time.Time, lastNo string) string {
	prefix := "JR" + day.Format("20060102")
	seq := 1
	lastNo = strings.TrimSpace(lastNo)
	if strings.HasPrefix(lastNo, prefix) && len(lastNo) >= len(prefix)+6 {
		var n int
		if _, err := fmt.Sscanf(lastNo[len(prefix):], "%d", &n); err == nil {
			seq = n + 1
		}
	}
	return fmt.Sprintf("%s%06d", prefix, seq)
}

func journalKeterangan(status, nip string) string {
	rawat := "JALAN"
	if strings.EqualFold(strings.TrimSpace(status), "ranap") {
		rawat = "INAP"
	}
	return fmt.Sprintf("PEMERIKSAAN LABORAT RAWAT %s, DIPOSTING OLEH %s", rawat, strings.TrimSpace(nip))
}

func loadLabCOA(ctx context.Context, tx *sql.Tx, status string) (labCOA, bool, error) {
	var coa labCOA
	var q string
	if strings.EqualFold(strings.TrimSpace(status), "ranap") {
		q = `
SELECT Suspen_Piutang_Laborat_Ranap, Laborat_Ranap,
       Beban_Jasa_Medik_Dokter_Laborat_Ranap, Utang_Jasa_Medik_Dokter_Laborat_Ranap,
       Beban_Jasa_Medik_Petugas_Laborat_Ranap, Utang_Jasa_Medik_Petugas_Laborat_Ranap,
       Beban_Kso_Laborat_Ranap, Utang_Kso_Laborat_Ranap,
       HPP_Persediaan_Laborat_Rawat_inap, Persediaan_BHP_Laborat_Rawat_Inap,
       Beban_Jasa_Sarana_Laborat_Ranap, Utang_Jasa_Sarana_Laborat_Ranap,
       Beban_Jasa_Perujuk_Laborat_Ranap, Utang_Jasa_Perujuk_Laborat_Ranap,
       Beban_Jasa_Menejemen_Laborat_Ranap, Utang_Jasa_Menejemen_Laborat_Ranap
FROM set_akun_ranap LIMIT 1`
	} else {
		q = `
SELECT Suspen_Piutang_Laborat_Ralan, Laborat_Ralan,
       Beban_Jasa_Medik_Dokter_Laborat_Ralan, Utang_Jasa_Medik_Dokter_Laborat_Ralan,
       Beban_Jasa_Medik_Petugas_Laborat_Ralan, Utang_Jasa_Medik_Petugas_Laborat_Ralan,
       Beban_Kso_Laborat_Ralan, Utang_Kso_Laborat_Ralan,
       HPP_Persediaan_Laborat_Rawat_Jalan, Persediaan_BHP_Laborat_Rawat_Jalan,
       Beban_Jasa_Sarana_Laborat_Ralan, Utang_Jasa_Sarana_Laborat_Ralan,
       Beban_Jasa_Perujuk_Laborat_Ralan, Utang_Jasa_Perujuk_Laborat_Ralan,
       Beban_Jasa_Menejemen_Laborat_Ralan, Utang_Jasa_Menejemen_Laborat_Ralan
FROM set_akun_ralan LIMIT 1`
	}
	err := tx.QueryRowContext(ctx, q).Scan(
		&coa.SuspenPiutang, &coa.Laborat,
		&coa.BebanJasaDokter, &coa.UtangJasaDokter,
		&coa.BebanJasaPetugas, &coa.UtangJasaPetugas,
		&coa.BebanKSO, &coa.UtangKSO,
		&coa.HPPPersediaan, &coa.PersediaanBHP,
		&coa.BebanJasaSarana, &coa.UtangJasaSarana,
		&coa.BebanJasaPerujuk, &coa.UtangJasaPerujuk,
		&coa.BebanMenejemen, &coa.UtangMenejemen,
	)
	if err == sql.ErrNoRows {
		return labCOA{}, false, nil
	}
	if err != nil {
		return labCOA{}, false, err
	}
	return coa, true, nil
}

func nextNoJurnal(ctx context.Context, tx *sql.Tx, day time.Time) (string, error) {
	prefix := "JR" + day.Format("20060102") + "%"
	var last sql.NullString
	err := tx.QueryRowContext(ctx, `
SELECT no_jurnal FROM jurnal WHERE no_jurnal LIKE ? ORDER BY no_jurnal DESC LIMIT 1`,
		prefix).Scan(&last)
	if err != nil && err != sql.ErrNoRows {
		return "", err
	}
	return formatNoJurnal(day, last.String), nil
}

func postLabJurnal(ctx context.Context, tx *sql.Tx, noRawat, nip, status string, totals journalTotals, now time.Time) error {
	if totals.isZero() {
		log.Printf("[medqlab]   db=jurnal SKIP totals=0")
		return nil
	}
	coa, ok, err := loadLabCOA(ctx, tx, status)
	if err != nil {
		return fmt.Errorf("load set_akun: %w", err)
	}
	if !ok {
		log.Printf("[medqlab]   db=jurnal SKIP set_akun missing status=%s", status)
		return nil
	}
	lines := buildJournalLines(coa, totals)
	if len(lines) == 0 {
		log.Printf("[medqlab]   db=jurnal SKIP no_lines")
		return nil
	}
	debet, kredit, balanced := journalBalanceOK(lines)
	if !balanced {
		return fmt.Errorf("jurnal unbalanced debet=%.2f kredit=%.2f", debet, kredit)
	}
	noJurnal, err := nextNoJurnal(ctx, tx, now)
	if err != nil {
		return fmt.Errorf("no_jurnal: %w", err)
	}
	ket := journalKeterangan(status, nip)
	tgl := now.Format("2006-01-02")
	jam := now.Format("15:04:05")
	if _, err := tx.ExecContext(ctx, `
INSERT INTO jurnal (no_jurnal, no_bukti, tgl_jurnal, jam_jurnal, jenis, keterangan)
VALUES (?,?,?,?,'U',?)`,
		noJurnal, noRawat, tgl, jam, ket); err != nil {
		return fmt.Errorf("insert jurnal: %w", err)
	}
	for _, l := range lines {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO detailjurnal (no_jurnal, kd_rek, debet, kredit) VALUES (?,?,?,?)`,
			noJurnal, l.KdRek, l.Debet, l.Kredit); err != nil {
			return fmt.Errorf("insert detailjurnal: %w", err)
		}
	}
	log.Printf("[medqlab]   db=jurnal INSERT no_jurnal=%s no_bukti=%s lines=%d debet=%.2f kredit=%.2f",
		noJurnal, noRawat, len(lines), debet, kredit)
	return nil
}
