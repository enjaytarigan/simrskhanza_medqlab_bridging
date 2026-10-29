package orderpoll

import (
	"encoding/json"
	"testing"
)

func TestMedQLabOrderKey(t *testing.T) {
	cases := []struct {
		full    string
		want    string
		wantErr bool
	}{
		{"PK202602250001", "2602250001", false},
		{"PL250923000123", "0923000123", false},
		{"PL2509230001", "", true}, // 12 chars — Java substring(4,14) throws
		{"ABC", "", true},
		{"", "", true},
	}
	for _, c := range cases {
		got, err := MedQLabOrderKey(c.full)
		if c.wantErr {
			if err == nil {
				t.Errorf("MedQLabOrderKey(%q) err=nil want error", c.full)
			}
			continue
		}
		if err != nil {
			t.Errorf("MedQLabOrderKey(%q) err=%v", c.full, err)
			continue
		}
		if got != c.want {
			t.Errorf("MedQLabOrderKey(%q)=%q want %q", c.full, got, c.want)
		}
	}
}

func TestIsInpatient(t *testing.T) {
	if !IsInpatient("ranap") || !IsInpatient("RANAP") || !IsInpatient(" Ranap ") {
		t.Fatal("ranap should be inpatient")
	}
	if IsInpatient("ralan") || IsInpatient("") || IsInpatient("ranap_lalu") {
		t.Fatal("non-ranap should be outpatient")
	}
}

func TestPatientType(t *testing.T) {
	cases := []struct {
		kdPJ      string
		inpatient bool
		wantID    string
		wantJenis string
	}{
		{"", false, "1", "UMUM"},
		{"  ", false, "1", "UMUM"},
		{"BPJ", false, "2", "ASURANSI"},
		{"-", false, "2", "ASURANSI"}, // ralan treats "-" as asuransi
		{"", true, "1", "UMUM"},
		{"-", true, "1", "UMUM"}, // ranap treats "-" as umum
		{"BPJ", true, "2", "ASURANSI"},
	}
	for _, c := range cases {
		id, jenis := patientType(c.kdPJ, c.inpatient)
		if id != c.wantID || jenis != c.wantJenis {
			t.Errorf("patientType(%q, inpatient=%v)=(%s,%s) want (%s,%s)",
				c.kdPJ, c.inpatient, id, jenis, c.wantID, c.wantJenis)
		}
	}
}

func TestCitoFlag(t *testing.T) {
	if citoFlag("CITO darah lengkap") != "true" {
		t.Fatal("expected cito true")
	}
	if citoFlag("pemeriksaan rutin") != "false" {
		t.Fatal("expected cito false")
	}
	if citoFlag("") != "false" {
		t.Fatal("empty info is not cito")
	}
}

func TestBuildPayloadRalan(t *testing.T) {
	h := Header{
		NoOrder:           "PK202602250001",
		NoRawat:           "2026/02/25/000001",
		NoRM:              "000022",
		NamaPasien:        "RUDI SANTOSO",
		TempatLahir:       "Jakarta",
		TglLahir:          "1957-03-11",
		JK:                "L",
		Alamat:            "Jl. Tes",
		KdPJ:              "BPJ",
		Penjamin:          "BPJS",
		InformasiTambahan: "CITO",
		DiagnosaKlinis:    "Demam",
		DokterPerujuk:     "D0000004",
		NamaDokter:        "dr. Hilyatul Nadia",
		KdPoli:            "INT",
		NmPoli:            "Penyakit Dalam",
	}
	panels := []Panel{
		{
			KdJenisPrw:  "106-A-K3",
			NmPerawatan: "Kimia Klinik",
			Templates: []OrderItem{
				{IDTest: "1002", NamaTest: "Leukosit"},
				{IDTest: "1001", NamaTest: "Hemoglobin"},
			},
		},
		{
			KdJenisPrw:  "004-A-K3",
			NmPerawatan: "Hematologi",
			Templates:   nil,
		},
	}
	body, err := BuildPayload(h, panels)
	if err != nil {
		t.Fatal(err)
	}
	if body.NoOrder != "2602250001" {
		t.Fatalf("no_order=%q", body.NoOrder)
	}
	if body.NoPendaftaran != h.NoRawat {
		t.Fatalf("no_pendaftaran=%q", body.NoPendaftaran)
	}
	if body.IDJenisPasien != "2" || body.JenisPasien != "ASURANSI" {
		t.Fatalf("jenis pasien=%s/%s", body.IDJenisPasien, body.JenisPasien)
	}
	if body.Cito != "true" {
		t.Fatalf("cito=%q", body.Cito)
	}
	if body.RujukanAsal != "1" {
		t.Fatalf("rujukan_asal=%q", body.RujukanAsal)
	}
	if body.ICD10 == nil || len(body.ICD10) != 0 {
		t.Fatalf("icd10 must be empty array, got %#v", body.ICD10)
	}
	if len(body.DetailRujukan) != 1 {
		t.Fatalf("detail_rujukan len=%d", len(body.DetailRujukan))
	}
	r := body.DetailRujukan[0]
	if r.IDWard != "INT" || r.Ward != "Penyakit Dalam" || r.InstallationType != "OUTPATIENT" {
		t.Fatalf("ralan rujukan=%+v", r)
	}
	wantOrder := []OrderItem{
		{IDTest: "106-A-K3", NamaTest: "Kimia Klinik"},
		{IDTest: "1002", NamaTest: "Leukosit"},
		{IDTest: "1001", NamaTest: "Hemoglobin"},
		{IDTest: "004-A-K3", NamaTest: "Hematologi"},
	}
	if len(body.Order) != len(wantOrder) {
		t.Fatalf("order len=%d want %d: %+v", len(body.Order), len(wantOrder), body.Order)
	}
	for i, item := range wantOrder {
		if body.Order[i] != item {
			t.Errorf("order[%d]=%+v want %+v", i, body.Order[i], item)
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, ok := decoded["phone"]; ok {
		t.Fatal("phone must not be present")
	}
	if _, ok := decoded["email"]; ok {
		t.Fatal("email must not be present")
	}
	if _, ok := decoded["nik"]; ok {
		t.Fatal("nik must not be present")
	}
	icd, ok := decoded["icd10"].([]any)
	if !ok || icd == nil || len(icd) != 0 {
		t.Fatalf("json icd10=%#v", decoded["icd10"])
	}
}

func TestBuildPayloadRanap(t *testing.T) {
	h := Header{
		NoOrder:       "PK202602250002",
		NoRawat:       "2026/02/25/000002",
		NoRM:          "000023",
		NamaPasien:    "SITI",
		TglLahir:      "1990-01-01",
		JK:            "P",
		KdPJ:          "-",
		Penjamin:      "-",
		DokterPerujuk: "D0000001",
		NamaDokter:    "dr. A",
		NmBangsal:     "Kelas I",
		KdKamar:       "I.01",
		Inpatient:     true,
	}
	panels := []Panel{{KdJenisPrw: "PK01", NmPerawatan: "GDS"}}
	body, err := BuildPayload(h, panels)
	if err != nil {
		t.Fatal(err)
	}
	if body.IDJenisPasien != "1" || body.JenisPasien != "UMUM" {
		t.Fatalf("ranap '-' must be UMUM, got %s/%s", body.IDJenisPasien, body.JenisPasien)
	}
	if body.Cito != "false" {
		t.Fatalf("cito=%q", body.Cito)
	}
	r := body.DetailRujukan[0]
	if r.IDWard != "Kelas I" || r.Ward != "I.01" || r.InstallationType != "INPATIENT" {
		t.Fatalf("ranap rujukan=%+v (Java swaps bangsal into id_ward)", r)
	}
}

func TestBuildPayloadRejectsShortNoorder(t *testing.T) {
	_, err := BuildPayload(Header{NoOrder: "PK2026"}, []Panel{{KdJenisPrw: "X", NmPerawatan: "X"}})
	if err == nil {
		t.Fatal("expected error for short noorder")
	}
}

func TestBuildPayloadRejectsEmptyOrder(t *testing.T) {
	_, err := BuildPayload(Header{NoOrder: "PK202602250001"}, nil)
	if err == nil {
		t.Fatal("expected error for empty order")
	}
}
