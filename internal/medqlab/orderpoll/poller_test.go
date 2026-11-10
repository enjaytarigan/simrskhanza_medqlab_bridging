package orderpoll

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"lis-khanza-mapper/internal/config"
)

func TestCanReprocess(t *testing.T) {
	cases := []struct {
		status string
		want   bool
	}{
		{"failed", true},
		{"FAILED", true},
		{" failed ", true},
		{"sent", false},
		{"claimed", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := CanReprocess(tc.status); got != tc.want {
			t.Fatalf("CanReprocess(%q)=%v want %v", tc.status, got, tc.want)
		}
	}
}

func TestNewProcessorRequiresCredentials(t *testing.T) {
	db := &sql.DB{}
	if p := NewProcessor(config.Config{MapperMode: config.ModeSIMRS}, db); p != nil {
		t.Fatal("expected nil without MedQLab credentials")
	}
	if p := NewProcessor(config.Config{
		MapperMode:     config.ModeSIMRS,
		MedQLabBaseURL: "https://lis.example",
		MedQLabXCons:   "c",
		MedQLabXSign:   "s",
	}, nil); p != nil {
		t.Fatal("expected nil without db")
	}
	if p := NewProcessor(config.Config{
		MapperMode:     config.ModeStandalone,
		MedQLabBaseURL: "https://lis.example",
		MedQLabXCons:   "c",
		MedQLabXSign:   "s",
	}, db); p != nil {
		t.Fatal("expected nil in standalone")
	}
	p := NewProcessor(config.Config{
		MapperMode:     config.ModeSIMRS,
		MedQLabBaseURL: "https://lis.example",
		MedQLabXCons:   "c",
		MedQLabXSign:   "s",
	}, db)
	if p == nil {
		t.Fatal("expected processor with credentials")
	}
}

func TestProcessOneNilAndEmpty(t *testing.T) {
	var nilP *Poller
	if err := nilP.ProcessOne(context.Background(), "PK1"); err == nil {
		t.Fatal("nil processor must error")
	}
	p := &Poller{}
	if err := p.ProcessOne(context.Background(), "  "); err == nil {
		t.Fatal("empty noorder must error")
	}
}

func TestNewDisabledWithoutFlag(t *testing.T) {
	p := New(config.Config{MapperMode: config.ModeSIMRS}, nil)
	if p != nil {
		t.Fatal("expected nil poller when OrderPoll is false")
	}
}

func TestNewDisabledStandalone(t *testing.T) {
	p := New(config.Config{
		MapperMode:        config.ModeStandalone,
		OrderPoll:         true,
		MedQLabBaseURL:    "https://lis.example",
		OrderPollInterval: time.Second,
		OrderClaimTimeout: time.Minute,
	}, nil)
	if p != nil {
		t.Fatal("expected nil poller in standalone mode")
	}
}

func TestNewEnabledSimrs(t *testing.T) {
	p := New(config.Config{
		MapperMode:        config.ModeSIMRS,
		OrderPoll:         true,
		MedQLabBaseURL:    "https://lis.example",
		MedQLabXCons:      "cons",
		MedQLabXSign:      "sign",
		OrderPollInterval: 5 * time.Second,
		OrderClaimTimeout: 2 * time.Minute,
	}, nil)
	if p != nil {
		t.Fatal("nil db must not start a poller")
	}
}

func TestClaimSQLWithoutRMFilter(t *testing.T) {
	stale := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	since := time.Date(2026, 10, 1, 14, 0, 0, 0, time.Local)
	query, args := claimSQL(nil, since, stale)
	if strings.Contains(query, "reg_periksa") || strings.Contains(query, "no_rkm_medis") {
		t.Fatalf("unfiltered claim must not join RM: %s", query)
	}
	if !strings.Contains(query, "pl.tgl_permintaan > ?") || !strings.Contains(query, "jam_permintaan") {
		t.Fatalf("missing datetime cutoff: %s", query)
	}
	if len(args) != 5 {
		t.Fatalf("args=%v", args)
	}
	if args[0] != "2026-10-01" || args[1] != "2026-10-01" || args[2] != "14:00:00" || args[3] != claimLabNo {
		t.Fatalf("args=%v", args)
	}
}

func TestClaimSQLWithRMAllowlist(t *testing.T) {
	stale := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	since := time.Date(2026, 10, 1, 0, 0, 0, 0, time.Local)
	query, args := claimSQL([]string{"000022", "000023"}, since, stale)
	if !strings.Contains(query, "INNER JOIN reg_periksa rp ON pl.no_rawat = rp.no_rawat") {
		t.Fatalf("missing reg_periksa join: %s", query)
	}
	if !strings.Contains(query, "AND rp.no_rkm_medis IN (?, ?)") {
		t.Fatalf("missing IN predicate: %s", query)
	}
	if len(args) != 7 {
		t.Fatalf("args=%v", args)
	}
	if args[5] != "000022" || args[6] != "000023" {
		t.Fatalf("RM args=%v", args[5:])
	}
}
