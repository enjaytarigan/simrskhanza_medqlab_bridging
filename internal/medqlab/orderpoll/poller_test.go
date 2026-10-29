package orderpoll

import (
	"strings"
	"testing"
	"time"

	"lis-khanza-mapper/internal/config"
)

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
	query, args := claimSQL(nil, "2026-10-01", stale)
	if strings.Contains(query, "reg_periksa") || strings.Contains(query, "no_rkm_medis") {
		t.Fatalf("unfiltered claim must not join RM: %s", query)
	}
	if len(args) != 3 {
		t.Fatalf("args=%v", args)
	}
	if args[0] != "2026-10-01" || args[1] != claimLabNo {
		t.Fatalf("args=%v", args)
	}
}

func TestClaimSQLWithRMAllowlist(t *testing.T) {
	stale := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	query, args := claimSQL([]string{"000022", "000023"}, "2026-10-01", stale)
	if !strings.Contains(query, "INNER JOIN reg_periksa rp ON pl.no_rawat = rp.no_rawat") {
		t.Fatalf("missing reg_periksa join: %s", query)
	}
	if !strings.Contains(query, "AND rp.no_rkm_medis IN (?, ?)") {
		t.Fatalf("missing IN predicate: %s", query)
	}
	if len(args) != 5 {
		t.Fatalf("args=%v", args)
	}
	if args[3] != "000022" || args[4] != "000023" {
		t.Fatalf("RM args=%v", args[3:])
	}
}
