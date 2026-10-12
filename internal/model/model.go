package model

import "time"

type LisTest struct {
	ID         uint64
	LisTestID  string
	LocalCode  string
	TestName   string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	MapCount   int
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
