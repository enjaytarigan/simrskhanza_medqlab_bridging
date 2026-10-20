package model

import "time"

type LisTest struct {
	ID        uint64
	LisTestID string
	LocalCode string
	TestName  string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
	MapCount  int
}

type Panel struct {
	KdJenisPrw    string
	NmPerawatan   string
	TotalByr      float64
	Kategori      string
	Status        string
	TemplateCount int
	SumBiayaItem  float64
	TarifMode     string // tindakan | template | none
}

type Template struct {
	IDTemplate  int
	Pemeriksaan string
	Satuan      string
	Urut        int
	KdJenisPrw  string
	NmPerawatan string
	BiayaItem   float64
	Mapped      bool
}

type Mapping struct {
	ID          uint64
	LisTestsPK  uint64
	LisTestID   string
	TestName    string
	IDTemplate  int
	Pemeriksaan string
	KdJenisPrw  string
	NmPerawatan string
	Status      string
	CreatedBy   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type BridgingLog struct {
	ID             uint64
	NoOrder        string
	NoLaboratorium string
	Status         string
	MappedCount    int
	UnmappedCount  int
	DetailWritten  int
	ErrorMessage   string
	PayloadJSON    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type BridgingLogListResult struct {
	Items []BridgingLog
	Total int
	Page  int
	Limit int
}

type DashboardStats struct {
	LisTestsActive int
	MappingsActive int
	TemplateCount  int
}

type BulkMapResult struct {
	Created int
	Updated int
	Skipped int
}

type CatalogRow struct {
	IDTemplate  int     `json:"id_template"`
	Pemeriksaan string  `json:"pemeriksaan"`
	Satuan      string  `json:"satuan"`
	Urut        int     `json:"urut"`
	KdJenisPrw  string  `json:"kd_jenis_prw"`
	NmPerawatan string  `json:"nm_perawatan"`
	BiayaItem   float64 `json:"biaya_item"`
	Kategori    string  `json:"kategori"`
	TotalByr    float64 `json:"total_byr"`
	Status      string  `json:"status"`
}

type CatalogImportResult struct {
	PanelsCreated    int      `json:"panels_created"`
	PanelsUpdated    int      `json:"panels_updated"`
	TemplatesCreated int      `json:"templates_created"`
	TemplatesUpdated int      `json:"templates_updated"`
	Skipped          int      `json:"skipped"`
	Errors           []string `json:"errors,omitempty"`
}
