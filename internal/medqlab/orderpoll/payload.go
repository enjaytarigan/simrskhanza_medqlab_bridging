package orderpoll

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Header is the SIMRS order + patient + referral fields used by saveOrder.
type Header struct {
	NoOrder           string
	NoRawat           string
	NoRM              string
	NamaPasien        string
	TempatLahir       string
	TglLahir          string
	JK                string
	Alamat            string
	KdPJ              string
	Penjamin          string
	InformasiTambahan string
	DiagnosaKlinis    string
	DokterPerujuk     string
	NamaDokter        string
	KdPoli            string
	NmPoli            string
	NmBangsal         string
	KdKamar           string
	Inpatient         bool
}

// Panel is one permintaan_pemeriksaan_lab row plus its templates.
type Panel struct {
	KdJenisPrw  string
	NmPerawatan string
	Templates   []OrderItem
}

// OrderItem is one saveOrder.order element (panel or template).
type OrderItem struct {
	IDTest   string `json:"id_test"`
	NamaTest string `json:"nama_test"`
}

// Rujukan is one detail_rujukan object.
type Rujukan struct {
	IDDokter         string `json:"id_dokter"`
	NamaDokter       string `json:"nama_dokter"`
	IDWard           string `json:"id_ward"`
	Ward             string `json:"ward"`
	InstallationType string `json:"installation_type"`
}

// SaveOrderBody matches ApiMEDQLAB kirimRalan / kirimRanap JSON.
type SaveOrderBody struct {
	NoOrder       string      `json:"no_order"`
	NoPendaftaran string      `json:"no_pendaftaran"`
	NoRM          string      `json:"no_rm"`
	NamaPasien    string      `json:"nama_pasien"`
	TempatLahir   string      `json:"tempat_lahir"`
	TglLahir      string      `json:"tgl_lahir"`
	JK            string      `json:"jk"`
	Alamat        string      `json:"alamat"`
	IDJenisPasien string      `json:"id_jenis_pasien"`
	JenisPasien   string      `json:"jenis_pasien"`
	RujukanAsal   string      `json:"rujukan_asal"`
	IDPenjamin    string      `json:"id_penjamin"`
	Penjamin      string      `json:"penjamin"`
	Cito          string      `json:"cito"`
	Diagnose      string      `json:"diagnose"`
	ICD10         []string    `json:"icd10"`
	DetailRujukan []Rujukan   `json:"detail_rujukan"`
	Order         []OrderItem `json:"order"`
}

// MedQLabOrderKey mirrors Java noorder.substring(4, 14).
// noorder shorter than 14 runes is rejected (Java throws StringIndexOutOfBoundsException).
func MedQLabOrderKey(noorder string) (string, error) {
	n := utf8.RuneCountInString(noorder)
	if n < 14 {
		return "", fmt.Errorf("noorder %q shorter than 14 characters", noorder)
	}
	runes := []rune(noorder)
	return string(runes[4:14]), nil
}

// IsInpatient is true when permintaan_lab.status is ranap (any case).
func IsInpatient(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "ranap")
}

func patientType(kdPJ string, inpatient bool) (id, jenis string) {
	k := strings.TrimSpace(kdPJ)
	if k == "" || (inpatient && k == "-") {
		return "1", "UMUM"
	}
	return "2", "ASURANSI"
}

func citoFlag(info string) string {
	if strings.Contains(strings.ToLower(info), "cito") {
		return "true"
	}
	return "false"
}

func flattenOrder(panels []Panel) []OrderItem {
	out := make([]OrderItem, 0)
	for _, p := range panels {
		out = append(out, OrderItem{IDTest: p.KdJenisPrw, NamaTest: p.NmPerawatan})
		out = append(out, p.Templates...)
	}
	return out
}

func rujukanFrom(h Header) Rujukan {
	r := Rujukan{
		IDDokter:   h.DokterPerujuk,
		NamaDokter: h.NamaDokter,
	}
	if h.Inpatient {
		r.IDWard = h.NmBangsal
		r.Ward = h.KdKamar
		r.InstallationType = "INPATIENT"
		return r
	}
	r.IDWard = h.KdPoli
	r.Ward = h.NmPoli
	r.InstallationType = "OUTPATIENT"
	return r
}

// BuildPayload builds the saveOrder body. Callers must already have rejected
// inpatient orders with no open kamar and orders with no examination items.
func BuildPayload(h Header, panels []Panel) (SaveOrderBody, error) {
	key, err := MedQLabOrderKey(h.NoOrder)
	if err != nil {
		return SaveOrderBody{}, err
	}
	idJenis, jenis := patientType(h.KdPJ, h.Inpatient)
	order := flattenOrder(panels)
	if len(order) == 0 {
		return SaveOrderBody{}, fmt.Errorf("order %q has no examination items", h.NoOrder)
	}
	return SaveOrderBody{
		NoOrder:       key,
		NoPendaftaran: h.NoRawat,
		NoRM:          h.NoRM,
		NamaPasien:    h.NamaPasien,
		TempatLahir:   h.TempatLahir,
		TglLahir:      h.TglLahir,
		JK:            h.JK,
		Alamat:        h.Alamat,
		IDJenisPasien: idJenis,
		JenisPasien:   jenis,
		RujukanAsal:   "1",
		IDPenjamin:    h.KdPJ,
		Penjamin:      h.Penjamin,
		Cito:          citoFlag(h.InformasiTambahan),
		Diagnose:      h.DiagnosaKlinis,
		ICD10:         []string{},
		DetailRujukan: []Rujukan{rujukanFrom(h)},
		Order:         order,
	}, nil
}
