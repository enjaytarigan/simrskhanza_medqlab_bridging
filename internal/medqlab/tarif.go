package medqlab

import (
	"context"
	"database/sql"
	"fmt"
)

// tarifMode classifies how Khanza bills a lab panel (DlgPeriksaLaboratorium).
type tarifMode string

const (
	tarifModeTindakan tarifMode = "tindakan"
	tarifModeTemplate tarifMode = "template"
	tarifModeNone     tarifMode = "none"
)

// loadOrderedTemplateTariffs returns template tariffs for items on the SIMRS order
// under a given panel (kd_jenis_prw).
func loadOrderedTemplateTariffs(ctx context.Context, tx *sql.Tx, noOrder, kdJenisPrw string) ([]templateTariff, error) {
	rows, err := tx.QueryContext(ctx, `
SELECT t.id_template, t.kd_jenis_prw,
       IFNULL(t.bagian_rs,0), IFNULL(t.bhp,0), IFNULL(t.bagian_perujuk,0),
       IFNULL(t.bagian_dokter,0), IFNULL(t.bagian_laborat,0),
       IFNULL(t.kso,0), IFNULL(t.menejemen,0), IFNULL(t.biaya_item,0)
FROM permintaan_detail_permintaan_lab pd
INNER JOIN template_laboratorium t ON t.id_template = pd.id_template
WHERE pd.noorder=? AND t.kd_jenis_prw=?`, noOrder, kdJenisPrw)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []templateTariff
	for rows.Next() {
		var t templateTariff
		if err := rows.Scan(
			&t.IDTemplate, &t.KdJenisPrw,
			&t.BagianRS, &t.BHP, &t.BagianPerujuk, &t.BagianDokter, &t.BagianLaborat,
			&t.KSO, &t.Menejemen, &t.BiayaItem,
		); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// sumTemplateTariffs rolls item tariffs into panel-shaped fields (used for mode detect / tests).
func sumTemplateTariffs(items []templateTariff) panelTariff {
	var p panelTariff
	for _, it := range items {
		p.BagianRS += it.BagianRS
		p.BHP += it.BHP
		p.TarifPerujuk += it.BagianPerujuk
		p.TarifTindakanDokter += it.BagianDokter
		p.TarifTindakanPetugas += it.BagianLaborat
		p.KSO += it.KSO
		p.Menejemen += it.Menejemen
		p.TotalByr += it.BiayaItem
	}
	return p
}

// resolvePanelTariff mirrors native Khanza billing (not AdamLabs Node rollup-to-panel):
//   tindakan — jns_perawatan_lab.total_byr > 0 → charge on periksa_lab, details 0
//   template — total_byr = 0 but ordered items have biaya_item → periksa_lab stays 0,
//              charge on detail_periksa_lab (Biaya Periksa = SUM items)
//   none     — both zero
func resolvePanelTariff(panel panelTariff, items []templateTariff) (panelTariff, tarifMode) {
	if panel.TotalByr > 0 {
		return panel, tarifModeTindakan
	}
	if sumTemplateTariffs(items).TotalByr > 0 {
		// Keep panel at master zeros; do not roll up onto periksa_lab.biaya.
		return panel, tarifModeTemplate
	}
	return panel, tarifModeNone
}

// detailTariffForMode: template mode keeps item tariffs; tindakan/none zero them
// so Biaya Periksa = panel XOR items, never both.
func detailTariffForMode(mode tarifMode, tpl templateTariff) templateTariff {
	if mode == tarifModeTemplate {
		return tpl
	}
	return templateTariff{IDTemplate: tpl.IDTemplate, KdJenisPrw: tpl.KdJenisPrw}
}

func updatePeriksaLabTariff(ctx context.Context, tx *sql.Tx, noRawat, kd, tgl, jam string, t panelTariff) error {
	_, err := tx.ExecContext(ctx, `
UPDATE periksa_lab SET
  bagian_rs=?, bhp=?, tarif_perujuk=?, tarif_tindakan_dokter=?, tarif_tindakan_petugas=?,
  kso=?, menejemen=?, biaya=?
WHERE no_rawat=? AND kd_jenis_prw=? AND tgl_periksa=? AND jam=? AND kategori='PK'`,
		t.BagianRS, t.BHP, t.TarifPerujuk, t.TarifTindakanDokter, t.TarifTindakanPetugas,
		t.KSO, t.Menejemen, t.TotalByr,
		noRawat, kd, tgl, jam,
	)
	if err != nil {
		return fmt.Errorf("update periksa_lab tarif %s: %w", kd, err)
	}
	return nil
}
