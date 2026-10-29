package config

import (
	"testing"
	"time"
)

func TestLoadOrderPollDisabledByDefault(t *testing.T) {
	t.Setenv("DATABASE_DSN", "user:pass@tcp(127.0.0.1:3306)/sik")
	t.Setenv("AUTH_USERNAME", "admin")
	t.Setenv("AUTH_PASSWORD", "secret")
	t.Setenv("MAPPER_MODE", "simrs")
	t.Setenv("MEDQLAB_ORDER_POLL", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OrderPoll {
		t.Fatal("order poll must be off by default")
	}
}

func TestLoadOrderPollRequiresFields(t *testing.T) {
	t.Setenv("DATABASE_DSN", "user:pass@tcp(127.0.0.1:3306)/sik")
	t.Setenv("AUTH_USERNAME", "admin")
	t.Setenv("AUTH_PASSWORD", "secret")
	t.Setenv("MAPPER_MODE", "simrs")
	t.Setenv("MEDQLAB_ORDER_POLL", "true")
	t.Setenv("MEDQLAB_BASE_URL", "")
	t.Setenv("MEDQLAB_X_CONS", "")
	t.Setenv("MEDQLAB_X_SIGN", "")
	t.Setenv("MEDQLAB_ORDER_SINCE", "")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when poll is on without URL/cons/sign")
	}

	t.Setenv("MEDQLAB_BASE_URL", "https://lis.example/ ")
	t.Setenv("MEDQLAB_X_CONS", "cons")
	t.Setenv("MEDQLAB_X_SIGN", "sign")
	if _, err := Load(); err == nil {
		t.Fatal("expected error when MEDQLAB_ORDER_SINCE is missing")
	}

	t.Setenv("MEDQLAB_ORDER_SINCE", "2026-10-01")
	t.Setenv("MEDQLAB_ORDER_INTERVAL", "3s")
	t.Setenv("MEDQLAB_ORDER_CLAIM_TIMEOUT", "90s")
	t.Setenv("APP_TIMEZONE", "Asia/Jakarta")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.OrderPoll {
		t.Fatal("order poll should be on")
	}
	if cfg.MedQLabBaseURL != "https://lis.example" {
		t.Fatalf("base URL=%q", cfg.MedQLabBaseURL)
	}
	if cfg.OrderPollInterval != 3*time.Second {
		t.Fatalf("interval=%s", cfg.OrderPollInterval)
	}
	if cfg.OrderClaimTimeout != 90*time.Second {
		t.Fatalf("claim timeout=%s", cfg.OrderClaimTimeout)
	}
	if cfg.OrderPollSince.Format("2006-01-02") != "2026-10-01" {
		t.Fatalf("since=%v", cfg.OrderPollSince)
	}
}

func TestLoadOrderPollRejectedInStandalone(t *testing.T) {
	t.Setenv("DATABASE_DSN", "user:pass@tcp(127.0.0.1:3306)/mapper")
	t.Setenv("AUTH_USERNAME", "admin")
	t.Setenv("AUTH_PASSWORD", "secret")
	t.Setenv("MAPPER_MODE", "standalone")
	t.Setenv("MEDQLAB_ORDER_POLL", "true")
	t.Setenv("MEDQLAB_BASE_URL", "https://lis.example")
	t.Setenv("MEDQLAB_X_CONS", "cons")
	t.Setenv("MEDQLAB_X_SIGN", "sign")
	t.Setenv("MEDQLAB_ORDER_SINCE", "2026-10-01")
	if _, err := Load(); err == nil {
		t.Fatal("standalone must not enable the order poller")
	}
}
