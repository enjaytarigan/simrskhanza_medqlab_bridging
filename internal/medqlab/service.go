package medqlab

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrOrderNotFound      = errors.New("permintaan_lab not found")
	ErrRegistrationClosed = errors.New("registrasi sudah ditutup")
	ErrNoMappedResults    = errors.New("no mapped examination results")
	ErrNoValidatedAt      = errors.New("no validatedAt on examination results")
	ErrNIPRequired        = errors.New("MEDQLAB_BRIDGING_NIP is required")
	ErrNoRawatRequired    = errors.New("visitNumber / no_rawat is required")
)

type Service struct {
	db  *sql.DB
	nip string
	loc *time.Location
}

func NewService(db *sql.DB, bridgingNIP string, loc *time.Location) *Service {
	if loc == nil {
		loc = time.Local
	}
	return &Service{db: db, nip: strings.TrimSpace(bridgingNIP), loc: loc}
}

type ProcessResult struct {
	InboxID        uint64 `json:"inbox_id"`
	MedQLabOrder   string `json:"medqlab_order"` // truncated key from MedQLab (SIMRS substring(4,14))
	NoOrder        string `json:"no_order"`      // full SIMRS permintaan_lab.noorder
	NoLaboratorium string `json:"no_laboratorium"`
	NoRawat        string `json:"no_rawat"` // from visitNumber / no_pendaftaran
	Mapped         int    `json:"mapped_count"`
	Unmapped       int    `json:"unmapped_count"`
	DetailWritten  int    `json:"detail_written"`
	PanelsWritten  int    `json:"panels_written"`
	TglPeriksa     string `json:"tgl_periksa"`
	JamPeriksa     string `json:"jam_periksa"`
}

type mappedRow struct {
	Leaf       LeafResult
	IDTemplate int
	KdJenisPrw string
}

type permintaanLab struct {
	NoOrder        string
	NoRawat        string
	DokterPerujuk  string
	Status         string
	TglHasil       string
	JamHasil       string
	TglPermintaan  string
	JamPermintaan  string
}

type panelTariff struct {
	BagianRS              float64
	BHP                   float64
	TarifPerujuk          float64
	TarifTindakanDokter   float64
	TarifTindakanPetugas  float64
	KSO                   float64
	Menejemen             float64
	TotalByr              float64
}

type templateTariff struct {
	IDTemplate    int
	KdJenisPrw    string
	BagianRS      float64
	BHP           float64
	BagianPerujuk float64
	BagianDokter  float64
	BagianLaborat float64
	KSO           float64
	Menejemen     float64
	BiayaItem     float64
}

// ProcessEnvelope validates, maps, and writes MedQLab results into SIMRS
// (periksa_lab, detail_periksa_lab, saran_kesan_lab) and posts lab journal
// for newly inserted tariff rows (mirrors Khanza / AdamLabs).
func (s *Service) ProcessEnvelope(ctx context.Context, env *Envelope, rawJSON []byte) (*ProcessResult, error) {
	start := time.Now()
	log.Printf("[medqlab] ========== ProcessEnvelope START ==========")

	if env == nil || env.Response == nil {
		log.Printf("[medqlab] FAIL: missing response")
		return nil, fmt.Errorf("missing response")
	}
	if env.MetaData != nil && env.MetaData.Code != 0 && env.MetaData.Code != 200 {
		log.Printf("[medqlab] FAIL: metaData.code=%d message=%s", env.MetaData.Code, env.MetaData.Message)
		return nil, fmt.Errorf("medqlab metaData.code=%d message=%s", env.MetaData.Code, env.MetaData.Message)
	}

	resp := env.Response
	// MedQLab always stores/returns the truncated key SIMRS sent (noorder.substring(4,14)).
	medqlabOrder := strings.TrimSpace(resp.NoOrder)
	if medqlabOrder == "" && resp.Demographics != nil {
		medqlabOrder = strings.TrimSpace(resp.Demographics.NoOrder)
	}
	if medqlabOrder == "" {
		log.Printf("[medqlab] FAIL: noOrder is required")
		return nil, fmt.Errorf("noOrder is required")
	}
	noLab := strings.TrimSpace(resp.NoLaboratorium)
	noRawat := extractNoRawat(resp)
	if noRawat == "" {
		log.Printf("[medqlab] FAIL: visitNumber/no_rawat missing (medqlab_order=%q)", medqlabOrder)
		return nil, ErrNoRawatRequired
	}

	log.Printf("[medqlab] step=identify medqlab_order=%q no_laboratorium=%q no_rawat=%q fallback_nip=%q payload_bytes=%d",
		medqlabOrder, noLab, noRawat, s.nip, len(rawJSON))

	inboxID, err := s.insertInbox(ctx, medqlabOrder, noLab, rawJSON)
	if err != nil {
		log.Printf("[medqlab] FAIL: insert lis_hasil_inbox: %v", err)
		return nil, fmt.Errorf("inbox: %w", err)
	}
	log.Printf("[medqlab] step=inbox_accepted inbox_id=%d", inboxID)

	result, err := s.process(ctx, medqlabOrder, noLab, noRawat, resp)
	if err != nil {
		log.Printf("[medqlab] step=process FAILED inbox_id=%d err=%v duration=%s",
			inboxID, err, time.Since(start).Round(time.Millisecond))
		_ = s.updateInbox(ctx, inboxID, "failed", 0, 0, 0, err.Error())
		return nil, err
	}
	result.InboxID = inboxID
	_ = s.updateInbox(ctx, inboxID, "posted", result.Mapped, result.Unmapped, result.DetailWritten, "")
	log.Printf("[medqlab] step=inbox_posted inbox_id=%d mapped=%d unmapped=%d details=%d",
		inboxID, result.Mapped, result.Unmapped, result.DetailWritten)
	log.Printf("[medqlab] ========== ProcessEnvelope OK duration=%s ==========",
		time.Since(start).Round(time.Millisecond))
	return result, nil
}

func (s *Service) process(ctx context.Context, medqlabOrder, noLab, noRawat string, resp *Response) (*ProcessResult, error) {
	log.Printf("[medqlab] step=resolve_permintaan medqlab_order=%q no_rawat=%q", medqlabOrder, noRawat)
	perm, err := s.resolvePermintaan(ctx, medqlabOrder, noRawat)
	if err != nil {
		log.Printf("[medqlab] step=resolve_permintaan FAIL: %v", err)
		return nil, err
	}
	noOrder := perm.NoOrder // full SIMRS noorder for all subsequent queries
	log.Printf("[medqlab] step=resolve_permintaan OK full_noorder=%q no_rawat=%q status=%q dokter_perujuk=%q tgl_hasil=%q jam_hasil=%q",
		perm.NoOrder, perm.NoRawat, perm.Status, perm.DokterPerujuk, perm.TglHasil, perm.JamHasil)

	closed, err := s.registrasiDitutup(ctx, perm.NoRawat)
	if err != nil {
		log.Printf("[medqlab] step=check_registrasi FAIL: %v", err)
		return nil, err
	}
	if closed {
		log.Printf("[medqlab] step=check_registrasi CLOSED no_rawat=%q (stts=Batal OR status_bayar=Sudah Bayar)", perm.NoRawat)
		return nil, ErrRegistrationClosed
	}
	log.Printf("[medqlab] step=check_registrasi OK no_rawat=%q still open", perm.NoRawat)

	leaves := FlattenLeaves(resp.Examinations)
	SortByPosition(leaves)
	log.Printf("[medqlab] step=flatten_examinations leaves=%d (from tree)", len(leaves))
	for i, leaf := range leaves {
		log.Printf("[medqlab]   leaf[%d] testId=%s name=%q position=%s nilai=%q flag=%q localCode=%s",
			i, leaf.LisTestID, leaf.TestName, leaf.Position, truncate(leaf.Nilai, 80), leaf.Keterangan, leaf.LocalCode)
	}

	if err := s.upsertLisTests(ctx, leaves); err != nil {
		log.Printf("[medqlab] step=upsert_lis_tests FAIL: %v", err)
		return nil, fmt.Errorf("upsert lis_tests: %w", err)
	}
	log.Printf("[medqlab] step=upsert_lis_tests OK")

	panels, err := s.loadOrderPanels(ctx, noOrder)
	if err != nil {
		log.Printf("[medqlab] step=load_order_panels FAIL: %v", err)
		return nil, err
	}
	log.Printf("[medqlab] step=load_order_panels count=%d kd=%v", len(panels), panels)

	orderedTemplates, err := s.loadOrderedTemplates(ctx, noOrder)
	if err != nil {
		log.Printf("[medqlab] step=load_ordered_templates FAIL: %v", err)
		return nil, err
	}
	log.Printf("[medqlab] step=load_ordered_templates count=%d", len(orderedTemplates))

	idTemplateByLis, err := s.loadMapping(ctx, leaves, panels)
	if err != nil {
		log.Printf("[medqlab] step=load_mapping FAIL: %v", err)
		return nil, err
	}
	log.Printf("[medqlab] step=load_mapping resolved=%d/%d testIds", len(idTemplateByLis), len(leaves))
	for lisID, idTpl := range idTemplateByLis {
		log.Printf("[medqlab]   map lis_test_id=%s -> id_template=%d", lisID, idTpl)
	}

	var mapped []mappedRow
	unmapped := 0
	for _, leaf := range leaves {
		idTpl, ok := idTemplateByLis[leaf.LisTestID]
		if !ok || idTpl == 0 {
			unmapped++
			log.Printf("[medqlab] step=map_leaf SKIP unmapped testId=%s name=%q", leaf.LisTestID, leaf.TestName)
			continue
		}
		if !orderedTemplates[idTpl] {
			unmapped++
			log.Printf("[medqlab] step=map_leaf SKIP testId=%s id_template=%d not on order %s", leaf.LisTestID, idTpl, noOrder)
			continue
		}
		kd, err := s.templateKdJenisPrw(ctx, idTpl)
		if err != nil || kd == "" {
			unmapped++
			log.Printf("[medqlab] step=map_leaf SKIP testId=%s id_template=%d missing kd_jenis_prw err=%v", leaf.LisTestID, idTpl, err)
			continue
		}
		mapped = append(mapped, mappedRow{Leaf: leaf, IDTemplate: idTpl, KdJenisPrw: kd})
		log.Printf("[medqlab] step=map_leaf OK testId=%s -> id_template=%d kd_jenis_prw=%s nilai=%q",
			leaf.LisTestID, idTpl, kd, truncate(leaf.Nilai, 80))
	}
	log.Printf("[medqlab] step=map_summary mapped=%d unmapped=%d", len(mapped), unmapped)
	if len(mapped) == 0 {
		log.Printf("[medqlab] FAIL: no mapped examination results")
		return nil, ErrNoMappedResults
	}

	tgl, jam, err := s.resolveExamDateTime(mapped)
	if err != nil {
		log.Printf("[medqlab] step=exam_datetime FAIL: %v", err)
		return nil, err
	}
	log.Printf("[medqlab] step=exam_datetime tgl=%s jam=%s source=max_validatedAt timezone=%s", tgl, jam, s.loc)

	nip, nipSource := s.resolveVerifyNIP(mapped)
	if nip == "" {
		log.Printf("[medqlab] FAIL: no idEmployeeVerify on mapped leaves and MEDQLAB_BRIDGING_NIP is empty")
		return nil, ErrNIPRequired
	}
	log.Printf("[medqlab] step=resolve_nip nip=%q source=%s", nip, nipSource)

	tglSampel, jamSampel := "", ""
	if resp.Demographics != nil {
		tglSampel, jamSampel = s.parseCollectDate(resp.Demographics.CollectDate)
	}
	if tglSampel != "" {
		log.Printf("[medqlab] step=sample_datetime tgl_sampel=%s jam_sampel=%s source=demographics.collectDate", tglSampel, jamSampel)
	} else {
		log.Printf("[medqlab] step=sample_datetime skip (collectDate empty/unparseable)")
	}

	pj, err := s.loadPJDokter(ctx)
	if err != nil {
		log.Printf("[medqlab] step=load_pj_dokter FAIL: %v", err)
		return nil, err
	}
	log.Printf("[medqlab] step=load_pj_dokter kd_dokterlab=%q", pj)

	kesan := ""
	if resp.Demographics != nil && resp.Demographics.CommentsSample != nil {
		kesan = strings.TrimSpace(*resp.Demographics.CommentsSample)
	}
	if utf8.RuneCountInString(kesan) > 700 {
		kesan = string([]rune(kesan)[:700])
	}
	if kesan != "" {
		log.Printf("[medqlab] step=kesan present len=%d preview=%q", utf8.RuneCountInString(kesan), truncate(kesan, 100))
	} else {
		log.Printf("[medqlab] step=kesan empty (skip saran_kesan_lab)")
	}

	log.Printf("[medqlab] step=write_simrs BEGIN noorder=%q no_rawat=%q rows=%d", noOrder, perm.NoRawat, len(mapped))
	panelsWritten, detailWritten, err := s.writeSIMRS(ctx, perm, mapped, tgl, jam, tglSampel, jamSampel, nip, pj, kesan)
	if err != nil {
		log.Printf("[medqlab] step=write_simrs FAIL: %v", err)
		return nil, err
	}
	log.Printf("[medqlab] step=write_simrs OK panels_inserted=%d details_written=%d", panelsWritten, detailWritten)

	return &ProcessResult{
		MedQLabOrder:   medqlabOrder,
		NoOrder:        noOrder,
		NoLaboratorium: noLab,
		NoRawat:        perm.NoRawat,
		Mapped:         len(mapped),
		Unmapped:       unmapped,
		DetailWritten:  detailWritten,
		PanelsWritten:  panelsWritten,
		TglPeriksa:     tgl,
		JamPeriksa:     jam,
	}, nil
}

// resolveExamDateTime sets permintaan_lab.tgl_hasil / jam_hasil (and periksa_lab /
// detail / saran_kesan datetime keys) from the latest examination validatedAt,
// converted to s.loc (APP_TIMEZONE). That wall-clock matches what SIMRS uses when
// printing hasil (tgl_hasil/jam_hasil and periksa_lab.tgl_periksa/jam).
func (s *Service) resolveExamDateTime(mapped []mappedRow) (tgl, jam string, err error) {
	var latest time.Time
	for _, m := range mapped {
		if m.Leaf.ValidatedAt == "" {
			continue
		}
		t, parseErr := parseMedQLabTime(m.Leaf.ValidatedAt, s.loc)
		if parseErr != nil {
			log.Printf("[medqlab] step=exam_datetime skip validatedAt=%q err=%v", m.Leaf.ValidatedAt, parseErr)
			continue
		}
		if t.After(latest) {
			latest = t
		}
	}
	if latest.IsZero() {
		return "", "", ErrNoValidatedAt
	}
	local := latest.In(s.loc)
	return local.Format("2006-01-02"), local.Format("15:04:05"), nil
}

// resolveVerifyNIP picks idEmployeeVerify from the mapped leaf with the latest verifiedAt.
// Falls back to MEDQLAB_BRIDGING_NIP (s.nip) when verify employee id is absent.
func (s *Service) resolveVerifyNIP(mapped []mappedRow) (nip, source string) {
	var latest time.Time
	var fromVerify string
	found := false
	for _, m := range mapped {
		if m.Leaf.VerifiedAt == "" {
			continue
		}
		t, parseErr := parseMedQLabTime(m.Leaf.VerifiedAt, s.loc)
		if parseErr != nil {
			log.Printf("[medqlab] step=resolve_nip skip verifiedAt=%q err=%v", m.Leaf.VerifiedAt, parseErr)
			continue
		}
		if !found || t.After(latest) {
			found = true
			latest = t
			fromVerify = strings.TrimSpace(m.Leaf.IdEmployeeVerify)
		}
	}
	if fromVerify != "" {
		return fromVerify, "idEmployeeVerify"
	}
	if fallback := strings.TrimSpace(s.nip); fallback != "" {
		return fallback, "env"
	}
	return "", ""
}

// parseCollectDate maps MedQLab demographics.collectDate → permintaan_lab tgl_sampel/jam_sampel.
func (s *Service) parseCollectDate(raw string) (tgl, jam string) {
	return parseCollectDateIn(raw, s.loc)
}

func parseCollectDateIn(raw string, loc *time.Location) (tgl, jam string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if loc == nil {
		loc = time.Local
	}
	t, err := parseMedQLabTime(raw, loc)
	if err != nil {
		log.Printf("[medqlab] parseCollectDate FAIL raw=%q err=%v", raw, err)
		return "", ""
	}
	local := t.In(loc)
	return local.Format("2006-01-02"), local.Format("15:04:05")
}

func parseMedQLabTime(raw string, loc *time.Location) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if loc == nil {
		loc = time.Local
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	// Naive datetime: interpret in configured hospital timezone.
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", raw, loc); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported datetime: %q", raw)
}

func (s *Service) writeSIMRS(ctx context.Context, perm *permintaanLab, mapped []mappedRow, tgl, jam, tglSampel, jamSampel, nip, pj, kesan string) (panelsWritten, detailWritten int, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if tglSampel != "" && jamSampel != "" {
		if _, err := tx.ExecContext(ctx, `
UPDATE permintaan_lab SET tgl_hasil=?, jam_hasil=?, tgl_sampel=?, jam_sampel=? WHERE noorder=?`,
			tgl, jam, tglSampel, jamSampel, perm.NoOrder); err != nil {
			return 0, 0, fmt.Errorf("update permintaan_lab: %w", err)
		}
		log.Printf("[medqlab]   db=permintaan_lab UPDATE tgl_hasil=%s jam_hasil=%s tgl_sampel=%s jam_sampel=%s WHERE noorder=%s",
			tgl, jam, tglSampel, jamSampel, perm.NoOrder)
	} else {
		if _, err := tx.ExecContext(ctx, `
UPDATE permintaan_lab SET tgl_hasil=?, jam_hasil=? WHERE noorder=?`,
			tgl, jam, perm.NoOrder); err != nil {
			return 0, 0, fmt.Errorf("update permintaan_lab: %w", err)
		}
		log.Printf("[medqlab]   db=permintaan_lab UPDATE tgl_hasil=%s jam_hasil=%s WHERE noorder=%s", tgl, jam, perm.NoOrder)
	}

	statusLabel := titleStatus(perm.Status)
	panelSet := map[string]struct{}{}
	for _, m := range mapped {
		panelSet[m.KdJenisPrw] = struct{}{}
	}
	log.Printf("[medqlab]   unique_panels=%d status_label=%s pj=%s nip=%s", len(panelSet), statusLabel, pj, nip)

	var totals journalTotals

	for kd := range panelSet {
		panel, err := loadPanelTariff(ctx, tx, kd)
		if err != nil {
			return 0, 0, fmt.Errorf("jns_perawatan_lab %s: %w", kd, err)
		}
		items, err := loadOrderedTemplateTariffs(ctx, tx, perm.NoOrder, kd)
		if err != nil {
			return 0, 0, fmt.Errorf("ordered templates %s: %w", kd, err)
		}
		tariff := resolvePanelTariff(panel, items)
		if panel.TotalByr <= 0 && tariff.TotalByr > 0 {
			log.Printf("[medqlab]   tarif_resolve kd=%s mode=template_rollup biaya=%.2f (panel_total_byr=0 items=%d)",
				kd, tariff.TotalByr, len(items))
		}

		exists, err := periksaLabExists(ctx, tx, perm.NoRawat, kd, tgl, jam)
		if err != nil {
			return 0, 0, err
		}
		if exists {
			if err := updatePeriksaLabTariff(ctx, tx, perm.NoRawat, kd, tgl, jam, tariff); err != nil {
				return 0, 0, err
			}
			log.Printf("[medqlab]   db=periksa_lab SKIP exists UPDATE_TARIF no_rawat=%s kd_jenis_prw=%s tgl=%s jam=%s biaya=%.2f",
				perm.NoRawat, kd, tgl, jam, tariff.TotalByr)
			continue
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO periksa_lab (
  no_rawat, nip, kd_jenis_prw, tgl_periksa, jam, dokter_perujuk,
  bagian_rs, bhp, tarif_perujuk, tarif_tindakan_dokter, tarif_tindakan_petugas,
  kso, menejemen, biaya, kd_dokter, status, kategori
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,'PK')`,
			perm.NoRawat, nip, kd, tgl, jam, perm.DokterPerujuk,
			tariff.BagianRS, tariff.BHP, tariff.TarifPerujuk, tariff.TarifTindakanDokter, tariff.TarifTindakanPetugas,
			tariff.KSO, tariff.Menejemen, tariff.TotalByr, pj, statusLabel,
		); err != nil {
			return 0, 0, fmt.Errorf("insert periksa_lab %s: %w", kd, err)
		}
		totals.addPanel(tariff)
		panelsWritten++
		log.Printf("[medqlab]   db=periksa_lab INSERT kd_jenis_prw=%s biaya=%.2f dokter_perujuk=%s",
			kd, tariff.TotalByr, perm.DokterPerujuk)
	}

	for _, m := range mapped {
		tpl, err := loadTemplateTariff(ctx, tx, m.IDTemplate)
		if err != nil {
			return 0, 0, fmt.Errorf("template %d: %w", m.IDTemplate, err)
		}
		nilai := truncate(m.Leaf.Nilai, 500)
		rujukan := truncate(m.Leaf.NilaiRujukan, 500)
		ket := truncate(m.Leaf.Keterangan, 200)

		action, err := upsertDetail(ctx, tx, perm.NoRawat, m.KdJenisPrw, tgl, jam, m.IDTemplate,
			nilai, rujukan, ket, tpl)
		if err != nil {
			return 0, 0, err
		}
		if action == "INSERT" {
			totals.addDetail(tpl)
		}
		if action != "" {
			detailWritten++
			log.Printf("[medqlab]   db=detail_periksa_lab %s id_template=%d kd=%s testId=%s name=%q nilai=%q ket=%q",
				action, m.IDTemplate, m.KdJenisPrw, m.Leaf.LisTestID, m.Leaf.TestName, truncate(nilai, 80), ket)
		}
	}

	if kesan != "" {
		kesanAction, err := upsertKesan(ctx, tx, perm.NoRawat, tgl, jam, kesan)
		if err != nil {
			return 0, 0, err
		}
		log.Printf("[medqlab]   db=saran_kesan_lab %s no_rawat=%s", kesanAction, perm.NoRawat)
	}

	if err := postLabJurnal(ctx, tx, perm.NoRawat, nip, perm.Status, totals, time.Now().In(s.loc)); err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}
	log.Printf("[medqlab]   db=COMMIT OK")
	return panelsWritten, detailWritten, nil
}

// upsertDetailUpdateSQL refreshes hasil and tariff columns from template_laboratorium
// (mirrors DlgPeriksaLaboratorium simpanlab on re-push).
const upsertDetailUpdateSQL = `
UPDATE detail_periksa_lab SET nilai=?, nilai_rujukan=?, keterangan=?,
  bagian_rs=?, bhp=?, bagian_perujuk=?, bagian_dokter=?, bagian_laborat=?,
  kso=?, menejemen=?, biaya_item=?
WHERE no_rawat=? AND kd_jenis_prw=? AND tgl_periksa=? AND jam=? AND id_template=?`

func upsertDetail(ctx context.Context, tx *sql.Tx, noRawat, kd, tgl, jam string, idTpl int,
	nilai, rujukan, ket string, tpl templateTariff) (action string, err error) {
	var n int
	err = tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM detail_periksa_lab
WHERE no_rawat=? AND kd_jenis_prw=? AND tgl_periksa=? AND jam=? AND id_template=?`,
		noRawat, kd, tgl, jam, idTpl).Scan(&n)
	if err != nil {
		return "", err
	}
	if n > 0 {
		_, err = tx.ExecContext(ctx, upsertDetailUpdateSQL,
			nilai, rujukan, ket,
			tpl.BagianRS, tpl.BHP, tpl.BagianPerujuk, tpl.BagianDokter, tpl.BagianLaborat,
			tpl.KSO, tpl.Menejemen, tpl.BiayaItem,
			noRawat, kd, tgl, jam, idTpl)
		if err != nil {
			return "", err
		}
		return "UPDATE", nil
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO detail_periksa_lab (
  no_rawat, kd_jenis_prw, tgl_periksa, jam, id_template,
  nilai, nilai_rujukan, keterangan,
  bagian_rs, bhp, bagian_perujuk, bagian_dokter, bagian_laborat, kso, menejemen, biaya_item
) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		noRawat, kd, tgl, jam, idTpl,
		nilai, rujukan, ket,
		tpl.BagianRS, tpl.BHP, tpl.BagianPerujuk, tpl.BagianDokter, tpl.BagianLaborat,
		tpl.KSO, tpl.Menejemen, tpl.BiayaItem,
	)
	if err != nil {
		return "", err
	}
	return "INSERT", nil
}

func upsertKesan(ctx context.Context, tx *sql.Tx, noRawat, tgl, jam, kesan string) (string, error) {
	var n int
	if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM saran_kesan_lab WHERE no_rawat=? AND tgl_periksa=? AND jam=?`,
		noRawat, tgl, jam).Scan(&n); err != nil {
		return "", err
	}
	if n > 0 {
		_, err := tx.ExecContext(ctx, `
UPDATE saran_kesan_lab SET kesan=? WHERE no_rawat=? AND tgl_periksa=? AND jam=?`,
			kesan, noRawat, tgl, jam)
		return "UPDATE", err
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO saran_kesan_lab (no_rawat, tgl_periksa, jam, saran, kesan) VALUES (?,?,?,'',?)`,
		noRawat, tgl, jam, kesan)
	return "INSERT", err
}

func periksaLabExists(ctx context.Context, tx *sql.Tx, noRawat, kd, tgl, jam string) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM periksa_lab
WHERE no_rawat=? AND kd_jenis_prw=? AND tgl_periksa=? AND jam=? AND kategori='PK'`,
		noRawat, kd, tgl, jam).Scan(&n)
	return n > 0, err
}

func loadPanelTariff(ctx context.Context, tx *sql.Tx, kd string) (panelTariff, error) {
	var t panelTariff
	err := tx.QueryRowContext(ctx, `
SELECT IFNULL(bagian_rs,0), IFNULL(bhp,0), IFNULL(tarif_perujuk,0),
       IFNULL(tarif_tindakan_dokter,0), IFNULL(tarif_tindakan_petugas,0),
       IFNULL(kso,0), IFNULL(menejemen,0), IFNULL(total_byr,0)
FROM jns_perawatan_lab WHERE kd_jenis_prw=?`, kd).Scan(
		&t.BagianRS, &t.BHP, &t.TarifPerujuk, &t.TarifTindakanDokter, &t.TarifTindakanPetugas,
		&t.KSO, &t.Menejemen, &t.TotalByr,
	)
	return t, err
}

func loadTemplateTariff(ctx context.Context, tx *sql.Tx, idTpl int) (templateTariff, error) {
	var t templateTariff
	err := tx.QueryRowContext(ctx, `
SELECT id_template, kd_jenis_prw,
       IFNULL(bagian_rs,0), IFNULL(bhp,0), IFNULL(bagian_perujuk,0),
       IFNULL(bagian_dokter,0), IFNULL(bagian_laborat,0),
       IFNULL(kso,0), IFNULL(menejemen,0), IFNULL(biaya_item,0)
FROM template_laboratorium WHERE id_template=?`, idTpl).Scan(
		&t.IDTemplate, &t.KdJenisPrw,
		&t.BagianRS, &t.BHP, &t.BagianPerujuk, &t.BagianDokter, &t.BagianLaborat,
		&t.KSO, &t.Menejemen, &t.BiayaItem,
	)
	return t, err
}

// extractNoRawat mirrors ApiMEDQLAB.getVisitNumber — SIMRS sent no_pendaftaran = no_rawat.
func extractNoRawat(resp *Response) string {
	if resp == nil {
		return ""
	}
	if resp.Demographics != nil {
		if v := strings.TrimSpace(resp.Demographics.VisitNumber); v != "" {
			return v
		}
		if v := strings.TrimSpace(resp.Demographics.VisitNumberSnake); v != "" {
			return v
		}
	}
	return strings.TrimSpace(resp.NoPendaftaran)
}

// resolvePermintaan maps MedQLab's truncated no_order + no_rawat to full SIMRS noorder.
//
// SIMRS always sends: no_order = noorder.substring(4,14), no_pendaftaran = no_rawat.
// Example:
//
//	full noorder  = PK202602250001
//	MedQLab key   = 2602250001          (substring(4,14))
//	no_rawat      = 2026/02/25/000001   (visitNumber)
//
// One no_rawat can have many orders, so both keys are required:
//
//	SUBSTRING(noorder, 5, 10) = medqlabOrder AND no_rawat = visitNumber
func (s *Service) resolvePermintaan(ctx context.Context, medqlabOrder, noRawat string) (*permintaanLab, error) {
	medqlabOrder = strings.TrimSpace(medqlabOrder)
	noRawat = strings.TrimSpace(noRawat)
	if medqlabOrder == "" {
		return nil, ErrOrderNotFound
	}
	if noRawat == "" {
		return nil, ErrNoRawatRequired
	}

	var p permintaanLab
	err := s.db.QueryRowContext(ctx, `
SELECT noorder, no_rawat, IFNULL(dokter_perujuk,''), IFNULL(status,''),
       IFNULL(tgl_hasil,''), IFNULL(jam_hasil,''),
       IFNULL(tgl_permintaan,''), IFNULL(jam_permintaan,'')
FROM permintaan_lab
WHERE SUBSTRING(noorder, 5, 10) = ? AND no_rawat = ?
LIMIT 1`, medqlabOrder, noRawat).Scan(
		&p.NoOrder, &p.NoRawat, &p.DokterPerujuk, &p.Status,
		&p.TglHasil, &p.JamHasil, &p.TglPermintaan, &p.JamPermintaan,
	)
	if errors.Is(err, sql.ErrNoRows) {
		log.Printf("[medqlab] step=resolve_permintaan NOT_FOUND SUBSTRING(noorder,5,10)=%q AND no_rawat=%q", medqlabOrder, noRawat)
		return nil, fmt.Errorf("%w: medqlab_order=%q no_rawat=%q", ErrOrderNotFound, medqlabOrder, noRawat)
	}
	if err != nil {
		return nil, err
	}
	log.Printf("[medqlab] step=resolve_permintaan matched full_noorder=%q", p.NoOrder)
	return &p, nil
}

func (s *Service) registrasiDitutup(ctx context.Context, noRawat string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM reg_periksa
WHERE no_rawat=? AND (stts='Batal' OR status_bayar='Sudah Bayar')`, noRawat).Scan(&n)
	return n > 0, err
}

func (s *Service) loadOrderPanels(ctx context.Context, noOrder string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT kd_jenis_prw FROM permintaan_pemeriksaan_lab WHERE noorder=?`, noOrder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var panels []string
	for rows.Next() {
		var kd string
		if err := rows.Scan(&kd); err != nil {
			return nil, err
		}
		kd = strings.TrimSpace(kd)
		if kd != "" {
			panels = append(panels, kd)
		}
	}
	return panels, rows.Err()
}

func (s *Service) loadOrderedTemplates(ctx context.Context, noOrder string) (map[int]bool, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id_template FROM permintaan_detail_permintaan_lab WHERE noorder=?`, noOrder)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

func (s *Service) loadMapping(ctx context.Context, leaves []LeafResult, panels []string) (map[string]int, error) {
	out := map[string]int{}
	if len(panels) == 0 || len(leaves) == 0 {
		return out, nil
	}
	testIDs := map[string]struct{}{}
	for _, l := range leaves {
		if l.LisTestID != "" {
			testIDs[l.LisTestID] = struct{}{}
		}
	}
	if len(testIDs) == 0 {
		return out, nil
	}

	args := make([]any, 0, len(testIDs)+len(panels))
	phTest := make([]string, 0, len(testIDs))
	for id := range testIDs {
		phTest = append(phTest, "?")
		args = append(args, id)
	}
	phPanel := make([]string, 0, len(panels))
	for _, p := range panels {
		phPanel = append(phPanel, "?")
		args = append(args, p)
	}

	q := fmt.Sprintf(`
SELECT t.lis_test_id, m.id_template
FROM lis_mapping_tests m
INNER JOIN lis_tests t ON t.id = m.lis_tests_pk
WHERE t.status='aktif' AND m.status='aktif'
  AND t.lis_test_id IN (%s)
  AND m.kd_jenis_prw IN (%s)
ORDER BY t.lis_test_id, m.updated_at DESC`,
		strings.Join(phTest, ","), strings.Join(phPanel, ","))

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var lisID string
		var idTpl int
		if err := rows.Scan(&lisID, &idTpl); err != nil {
			return nil, err
		}
		if _, exists := out[lisID]; !exists {
			out[lisID] = idTpl
		}
	}
	return out, rows.Err()
}

func (s *Service) templateKdJenisPrw(ctx context.Context, idTpl int) (string, error) {
	var kd string
	err := s.db.QueryRowContext(ctx, `
SELECT kd_jenis_prw FROM template_laboratorium WHERE id_template=?`, idTpl).Scan(&kd)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return strings.TrimSpace(kd), err
}

func (s *Service) loadPJDokter(ctx context.Context) (string, error) {
	var kd sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT kd_dokterlab FROM set_pjlab LIMIT 1`).Scan(&kd)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("set_pjlab.kd_dokterlab not configured")
	}
	if err != nil {
		return "", err
	}
	if !kd.Valid || strings.TrimSpace(kd.String) == "" {
		return "", fmt.Errorf("set_pjlab.kd_dokterlab is empty")
	}
	return strings.TrimSpace(kd.String), nil
}

func (s *Service) upsertLisTests(ctx context.Context, leaves []LeafResult) error {
	uniq := map[string]LeafResult{}
	for _, l := range leaves {
		if l.LisTestID == "" {
			continue
		}
		uniq[l.LisTestID] = l
	}
	for _, l := range uniq {
		if _, err := s.db.ExecContext(ctx, `
INSERT INTO lis_tests (lis_test_id, local_code, test_name, status)
VALUES (?,?,?,'aktif')
ON DUPLICATE KEY UPDATE local_code=VALUES(local_code), test_name=VALUES(test_name), updated_at=NOW()`,
			l.LisTestID, l.LocalCode, l.TestName); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) insertInbox(ctx context.Context, noOrder, noLab string, raw []byte) (uint64, error) {
	payload := string(raw)
	if !json.Valid(raw) {
		payload = ""
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO lis_hasil_inbox (no_order, no_laboratorium, status, payload_json)
VALUES (?,?, 'accepted', ?)`, noOrder, noLab, payload)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	return uint64(id), err
}

func (s *Service) updateInbox(ctx context.Context, id uint64, status string, mapped, unmapped, written int, errMsg string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE lis_hasil_inbox
SET status=?, mapped_count=?, unmapped_count=?, detail_written=?, error_message=?, updated_at=NOW()
WHERE id=?`, status, mapped, unmapped, written, nullIfEmpty(errMsg), id)
	return err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func titleStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch s {
	case "ranap":
		return "Ranap"
	case "ralan":
		return "Ralan"
	default:
		if s == "" {
			return "Ralan"
		}
		return strings.ToUpper(s[:1]) + s[1:]
	}
}

func truncate(s string, max int) string {
	if max <= 0 || utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max])
}
