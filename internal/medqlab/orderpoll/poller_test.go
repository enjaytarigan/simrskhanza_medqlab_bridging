package orderpoll

import (
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
