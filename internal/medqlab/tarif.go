package medqlab

import (
	"context"
	"database/sql"
	"fmt"
)

// tarifMode mirrors AdamLabs Node resolveTindakanTarif modes.
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

// sumTemplateTariffs rolls item tariffs into panel-shaped fields
// (AdamLabs Node resolveTindakanTarif mode=template).
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

// resolvePanelTariff mirrors AdamLabs Node resolveTindakanTarif:
// use panel master when TotalByr > 0; otherwise roll up ordered template biaya_item.
func resolvePanelTariff(panel panelTariff, items []templateTariff) (panelTariff, tarifMode) {
	if panel.TotalByr > 0 {
		return panel, tarifModeTindakan
	}
	summed := sumTemplateTariffs(items)
	if summed.TotalByr > 0 {
		return summed, tarifModeTemplate
	}
	return panel, tarifModeNone
}

// detailTariffForMode always zeros detail billing columns. Charge lives once on
// periksa_lab (tindakan master or template rollup). Keeping template biaya_item
// on details made Khanza Biaya Periksa = panel + SUM(items) (double-count), e.g.
// 002-A-K3 with total_byr=0 and 3×40k items → 120k panel + 120k details = 240k.
func detailTariffForMode(mode tarifMode, tpl templateTariff) templateTariff {
	_ = mode // mode still used for panel resolve / logging; details never billed twice
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
