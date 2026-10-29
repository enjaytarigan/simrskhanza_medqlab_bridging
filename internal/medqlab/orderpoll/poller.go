package orderpoll

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"lis-khanza-mapper/internal/config"
)

const claimLabNo = "00"

// Poller claims one permintaan_lab at a time, POSTs saveOrder, and records the result.
type Poller struct {
	db       *sql.DB
	client   *Client
	since    time.Time
	interval time.Duration
	claimTTL time.Duration
	onlyRM   []string
}

func New(cfg config.Config, db *sql.DB) *Poller {
	if !cfg.OrderPoll || cfg.Standalone() || db == nil {
		return nil
	}
	return &Poller{
		db:       db,
		client:   NewClient(cfg.MedQLabBaseURL, cfg.MedQLabXCons, cfg.MedQLabXSign),
		since:    cfg.OrderPollSince,
		interval: cfg.OrderPollInterval,
		claimTTL: cfg.OrderClaimTimeout,
		onlyRM:   append([]string(nil), cfg.OrderOnlyRM...),
	}
}

func (p *Poller) Run(ctx context.Context) {
	if p == nil {
		return
	}
	rmLabel := "all patients"
	if len(p.onlyRM) > 0 {
		rmLabel = "order_only_rm=" + strings.Join(p.onlyRM, ",")
	}
	log.Printf("[orderpoll] started interval=%s since=%s claim_timeout=%s %s",
		p.interval, p.since.Format("2006-01-02"), p.claimTTL, rmLabel)
	p.tick(ctx)
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[orderpoll] stopped")
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

func (p *Poller) tick(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	noorder, err := p.claimNext(ctx)
	if err != nil {
		log.Printf("[orderpoll] claim: %v", err)
		return
	}
	if noorder == "" {
		return
	}
	if err := p.process(ctx, noorder); err != nil {
		log.Printf("[orderpoll] process noorder=%s: %v", noorder, err)
	}
}

func (p *Poller) claimNext(ctx context.Context) (string, error) {
	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	staleBefore := time.Now().Add(-p.claimTTL)
	query, args := claimSQL(p.onlyRM, p.since.Format("2006-01-02"), staleBefore)
	var noorder string
	err = tx.QueryRowContext(ctx, query, args...).Scan(&noorder)
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return "", nil
	}
	if err != nil {
		return "", err
	}

	_, err = tx.ExecContext(ctx, `
INSERT INTO lis_order_outbox (noorder, no_laboratorium, status, claimed_at)
VALUES (?, ?, 'claimed', NOW())
ON DUPLICATE KEY UPDATE
  no_laboratorium = VALUES(no_laboratorium),
  status = 'claimed',
  claimed_at = NOW(),
  error_message = NULL,
  sent_at = NULL,
  payload_json = NULL`, noorder, claimLabNo)
	if err != nil {
		return "", fmt.Errorf("claim outbox %s: %w", noorder, err)
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return noorder, nil
}

func claimSQL(onlyRM []string, since string, staleBefore time.Time) (string, []any) {
	join := ""
	rmFilter := ""
	args := []any{since, claimLabNo, staleBefore}
	if len(onlyRM) > 0 {
		join = "INNER JOIN reg_periksa rp ON pl.no_rawat = rp.no_rawat"
		placeholders := make([]string, len(onlyRM))
		for i, rm := range onlyRM {
			placeholders[i] = "?"
			args = append(args, rm)
		}
		rmFilter = "AND rp.no_rkm_medis IN (" + strings.Join(placeholders, ", ") + ")"
	}
	query := fmt.Sprintf(`
SELECT pl.noorder
FROM permintaan_lab pl
%s
LEFT JOIN lis_order_outbox o ON o.noorder = pl.noorder
WHERE pl.tgl_permintaan >= ?
  AND (
    o.noorder IS NULL
    OR (o.status = 'claimed' AND o.no_laboratorium = ? AND o.claimed_at <= ?)
  )
  %s
ORDER BY pl.tgl_permintaan ASC, pl.jam_permintaan ASC
LIMIT 1
FOR UPDATE`, join, rmFilter)
	return query, args
}

func (p *Poller) process(ctx context.Context, noorder string) error {
	header, err := p.loadHeader(ctx, noorder)
	if err != nil {
		_ = p.markFailed(ctx, noorder, "", nil, err.Error())
		return err
	}
	panels, err := p.loadPanels(ctx, noorder)
	if err != nil {
		_ = p.markFailed(ctx, noorder, header.NoRawat, nil, err.Error())
		return err
	}
	body, err := BuildPayload(header, panels)
	if err != nil {
		_ = p.markFailed(ctx, noorder, header.NoRawat, nil, err.Error())
		return err
	}
	payloadJSON, _ := json.Marshal(body)
	log.Printf("[orderpoll] send noorder=%s no_order=%s no_rawat=%s inpatient=%v items=%d",
		noorder, body.NoOrder, body.NoPendaftaran, header.Inpatient, len(body.Order))

	labNo, err := p.client.SaveOrder(ctx, body, header.Inpatient)
	if err != nil {
		_ = p.markFailed(ctx, noorder, header.NoRawat, payloadJSON, err.Error())
		return err
	}
	if err := p.markSent(ctx, noorder, header.NoRawat, labNo, payloadJSON); err != nil {
		return err
	}
	log.Printf("[orderpoll] sent noorder=%s no_laboratorium=%s", noorder, labNo)
	return nil
}

func (p *Poller) loadHeader(ctx context.Context, noorder string) (Header, error) {
	var status string
	err := p.db.QueryRowContext(ctx,
		`SELECT IFNULL(status,'') FROM permintaan_lab WHERE noorder=?`, noorder).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return Header{}, fmt.Errorf("permintaan_lab not found")
	}
	if err != nil {
		return Header{}, err
	}
	if IsInpatient(status) {
		h, err := p.loadRanapHeader(ctx, noorder)
		if err != nil {
			return Header{}, err
		}
		h.Inpatient = true
		return h, nil
	}
	return p.loadRalanHeader(ctx, noorder)
}

func (p *Poller) loadRalanHeader(ctx context.Context, noorder string) (Header, error) {
	var h Header
	err := p.db.QueryRowContext(ctx, `
SELECT
  pl.noorder,
  pl.no_rawat,
  rp.no_rkm_medis,
  p.nm_pasien,
  IFNULL(p.tmp_lahir,''),
  IFNULL(DATE_FORMAT(p.tgl_lahir, '%Y-%m-%d'),''),
  IFNULL(p.jk,''),
  IFNULL(p.alamat,''),
  IFNULL(rp.kd_pj,''),
  IFNULL(pj.png_jawab,''),
  IFNULL(pl.informasi_tambahan,''),
  IFNULL(pl.diagnosa_klinis,''),
  IFNULL(pl.dokter_perujuk,''),
  IFNULL(d.nm_dokter,''),
  IFNULL(rp.kd_poli,''),
  IFNULL(pol.nm_poli,'')
FROM permintaan_lab pl
INNER JOIN reg_periksa rp ON pl.no_rawat = rp.no_rawat
INNER JOIN pasien p ON rp.no_rkm_medis = p.no_rkm_medis
INNER JOIN dokter d ON pl.dokter_perujuk = d.kd_dokter
INNER JOIN penjab pj ON rp.kd_pj = pj.kd_pj
INNER JOIN poliklinik pol ON rp.kd_poli = pol.kd_poli
WHERE pl.noorder = ?`, noorder).Scan(
		&h.NoOrder, &h.NoRawat, &h.NoRM, &h.NamaPasien, &h.TempatLahir, &h.TglLahir,
		&h.JK, &h.Alamat, &h.KdPJ, &h.Penjamin, &h.InformasiTambahan, &h.DiagnosaKlinis,
		&h.DokterPerujuk, &h.NamaDokter, &h.KdPoli, &h.NmPoli,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Header{}, fmt.Errorf("ralan header not found")
	}
	return h, err
}

func (p *Poller) loadRanapHeader(ctx context.Context, noorder string) (Header, error) {
	var h Header
	err := p.db.QueryRowContext(ctx, `
SELECT
  pl.noorder,
  pl.no_rawat,
  rp.no_rkm_medis,
  p.nm_pasien,
  IFNULL(p.tmp_lahir,''),
  IFNULL(DATE_FORMAT(p.tgl_lahir, '%Y-%m-%d'),''),
  IFNULL(p.jk,''),
  IFNULL(p.alamat,''),
  IFNULL(rp.kd_pj,''),
  IFNULL(pj.png_jawab,''),
  IFNULL(pl.informasi_tambahan,''),
  IFNULL(pl.diagnosa_klinis,''),
  IFNULL(pl.dokter_perujuk,''),
  IFNULL(d.nm_dokter,''),
  IFNULL(b.nm_bangsal,''),
  IFNULL(ki.kd_kamar,'')
FROM permintaan_lab pl
INNER JOIN reg_periksa rp ON pl.no_rawat = rp.no_rawat
INNER JOIN pasien p ON rp.no_rkm_medis = p.no_rkm_medis
INNER JOIN dokter d ON pl.dokter_perujuk = d.kd_dokter
INNER JOIN penjab pj ON rp.kd_pj = pj.kd_pj
INNER JOIN kamar_inap ki ON rp.no_rawat = ki.no_rawat AND ki.stts_pulang = '-'
INNER JOIN kamar k ON ki.kd_kamar = k.kd_kamar
INNER JOIN bangsal b ON k.kd_bangsal = b.kd_bangsal
WHERE pl.noorder = ?
ORDER BY ki.tgl_masuk DESC, ki.jam_masuk DESC
LIMIT 1`, noorder).Scan(
		&h.NoOrder, &h.NoRawat, &h.NoRM, &h.NamaPasien, &h.TempatLahir, &h.TglLahir,
		&h.JK, &h.Alamat, &h.KdPJ, &h.Penjamin, &h.InformasiTambahan, &h.DiagnosaKlinis,
		&h.DokterPerujuk, &h.NamaDokter, &h.NmBangsal, &h.KdKamar,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Header{}, fmt.Errorf("kamar ranap tidak ketemu")
	}
	return h, err
}

func (p *Poller) loadPanels(ctx context.Context, noorder string) ([]Panel, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT pp.kd_jenis_prw, jpl.nm_perawatan
FROM permintaan_pemeriksaan_lab pp
INNER JOIN jns_perawatan_lab jpl ON pp.kd_jenis_prw = jpl.kd_jenis_prw
WHERE pp.noorder = ?
ORDER BY pp.kd_jenis_prw DESC`, noorder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var panels []Panel
	for rows.Next() {
		var pan Panel
		if err := rows.Scan(&pan.KdJenisPrw, &pan.NmPerawatan); err != nil {
			return nil, err
		}
		tpls, err := p.loadTemplates(ctx, noorder, pan.KdJenisPrw)
		if err != nil {
			return nil, err
		}
		pan.Templates = tpls
		panels = append(panels, pan)
	}
	return panels, rows.Err()
}

func (p *Poller) loadTemplates(ctx context.Context, noorder, kdJenisPrw string) ([]OrderItem, error) {
	rows, err := p.db.QueryContext(ctx, `
SELECT CAST(pd.id_template AS CHAR), tl.Pemeriksaan
FROM permintaan_detail_permintaan_lab pd
INNER JOIN template_laboratorium tl ON pd.id_template = tl.id_template
WHERE pd.noorder = ? AND pd.kd_jenis_prw = ?
ORDER BY tl.urut DESC`, noorder, kdJenisPrw)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrderItem
	for rows.Next() {
		var item OrderItem
		if err := rows.Scan(&item.IDTest, &item.NamaTest); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (p *Poller) markSent(ctx context.Context, noorder, noRawat, labNo string, payload []byte) error {
	_, err := p.db.ExecContext(ctx, `
UPDATE lis_order_outbox SET
  status = 'sent',
  no_rawat = ?,
  no_laboratorium = ?,
  payload_json = ?,
  error_message = NULL,
  sent_at = NOW()
WHERE noorder = ?`, noRawat, labNo, nullableJSON(payload), noorder)
	return err
}

func (p *Poller) markFailed(ctx context.Context, noorder, noRawat string, payload []byte, message string) error {
	message = strings.TrimSpace(message)
	if len(message) > 4000 {
		message = message[:4000]
	}
	_, err := p.db.ExecContext(ctx, `
UPDATE lis_order_outbox SET
  status = 'failed',
  no_rawat = ?,
  no_laboratorium = '',
  payload_json = ?,
  error_message = ?,
  sent_at = NULL
WHERE noorder = ?`, noRawat, nullableJSON(payload), message, noorder)
	return err
}

func nullableJSON(payload []byte) any {
	if len(payload) == 0 {
		return nil
	}
	return string(payload)
}
