package catalog

import (
	"strings"
	"testing"
)

func TestParseCSV(t *testing.T) {
	raw := "id_template,pemeriksaan,satuan,urut,kd_jenis_prw,nm_perawatan,biaya_item,kategori,total_byr,status\n" +
		"1001,Hemoglobin,g/dL,1,PK-01,Hematologi,40000,PK,0,1\n"
	rows, err := ParseCSV(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	r := rows[0]
	if r.IDTemplate != 1001 || r.KdJenisPrw != "PK-01" || r.Pemeriksaan != "Hemoglobin" {
		t.Fatalf("unexpected row: %+v", r)
	}
	if r.BiayaItem != 40000 {
		t.Fatalf("biaya_item=%v", r.BiayaItem)
	}
}

func TestParseCSVBOMAndCase(t *testing.T) {
	raw := "\ufeffID_TEMPLATE,Pemeriksaan,kd_jenis_prw\n42,Natrium,EL-01\n"
	rows, err := ParseCSV(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].IDTemplate != 42 || rows[0].KdJenisPrw != "EL-01" {
		t.Fatalf("got %+v", rows)
	}
}

func TestParseCSVMissingColumn(t *testing.T) {
	_, err := ParseCSV(strings.NewReader("id_template,pemeriksaan\n1,Hb\n"))
	if err == nil {
		t.Fatal("expected error")
	}
}
