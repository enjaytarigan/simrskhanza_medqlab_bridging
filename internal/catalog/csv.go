package catalog

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"

	"lis-khanza-mapper/internal/model"
)

var csvHeaders = []string{
	"id_template", "pemeriksaan", "satuan", "urut", "kd_jenis_prw",
	"nm_perawatan", "biaya_item", "kategori", "total_byr", "status",
}

func ParseCSV(r io.Reader) ([]model.CatalogRow, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	cr.ReuseRecord = true
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("csv header: %w", err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\ufeff")
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := idx["id_template"]; !ok {
		return nil, fmt.Errorf("csv missing column id_template")
	}
	if _, ok := idx["kd_jenis_prw"]; !ok {
		return nil, fmt.Errorf("csv missing column kd_jenis_prw")
	}

	var rows []model.CatalogRow
	line := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv line %d: %w", line+1, err)
		}
		line++
		get := func(name string) string {
			i, ok := idx[name]
			if !ok || i >= len(rec) {
				return ""
			}
			return strings.TrimSpace(rec[i])
		}
		idTpl, err := atoiDefault(get("id_template"), 0)
		if err != nil {
			return nil, fmt.Errorf("csv line %d: id_template: %w", line, err)
		}
		urut, err := atoiDefault(get("urut"), 0)
		if err != nil {
			return nil, fmt.Errorf("csv line %d: urut: %w", line, err)
		}
		biaya, err := atofDefault(get("biaya_item"), 0)
		if err != nil {
			return nil, fmt.Errorf("csv line %d: biaya_item: %w", line, err)
		}
		total, err := atofDefault(get("total_byr"), 0)
		if err != nil {
			return nil, fmt.Errorf("csv line %d: total_byr: %w", line, err)
		}
		rows = append(rows, model.CatalogRow{
			IDTemplate:  idTpl,
			Pemeriksaan: get("pemeriksaan"),
			Satuan:      get("satuan"),
			Urut:        urut,
			KdJenisPrw:  get("kd_jenis_prw"),
			NmPerawatan: get("nm_perawatan"),
			BiayaItem:   biaya,
			Kategori:    get("kategori"),
			TotalByr:    total,
			Status:      get("status"),
		})
	}
	return rows, nil
}

func HeaderLine() string {
	return strings.Join(csvHeaders, ",")
}

func atoiDefault(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func atofDefault(s string, def float64) (float64, error) {
	if s == "" {
		return def, nil
	}
	s = strings.ReplaceAll(s, ",", "")
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}
