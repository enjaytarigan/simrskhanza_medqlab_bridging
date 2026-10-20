package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ModeSIMRS      = "simrs"
	ModeStandalone = "standalone"
)

type Config struct {
	DatabaseDSN  string
	AuthUsername string
	AuthPassword string
	Listen       string
	Env          string
	MapperMode   string

	// Timezone is the IANA name used for MedQLab datetime → SIMRS wall-clock conversion
	// (e.g. Asia/Jakarta, Asia/Makassar, Asia/Jayapura, Asia/Pontianak).
	// Empty means the process local timezone (time.Local).
	Timezone string
	Location *time.Location

	// MedQLab push webhook (optional — endpoint returns 503 if API key unset).
	MedQLabWebhookAPIKey string
	MedQLabBridgingNIP   string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseDSN:          strings.TrimSpace(os.Getenv("DATABASE_DSN")),
		AuthUsername:         strings.TrimSpace(os.Getenv("AUTH_USERNAME")),
		AuthPassword:         os.Getenv("AUTH_PASSWORD"),
		Listen:               envOr("APP_LISTEN", ":8080"),
		Env:                  envOr("APP_ENV", "production"),
		MedQLabWebhookAPIKey: strings.TrimSpace(os.Getenv("MEDQLAB_WEBHOOK_API_KEY")),
		MedQLabBridgingNIP:   strings.TrimSpace(os.Getenv("MEDQLAB_BRIDGING_NIP")),
		MapperMode:           strings.ToLower(envOr("MAPPER_MODE", ModeSIMRS)),
	}
	if cfg.MapperMode != ModeSIMRS && cfg.MapperMode != ModeStandalone {
		return cfg, fmt.Errorf("MAPPER_MODE must be %s or %s", ModeSIMRS, ModeStandalone)
	}
	if cfg.DatabaseDSN == "" {
		return cfg, fmt.Errorf("DATABASE_DSN is required")
	}
	if cfg.AuthUsername == "" || cfg.AuthPassword == "" {
		return cfg, fmt.Errorf("AUTH_USERNAME and AUTH_PASSWORD are required")
	}

	tzName := strings.TrimSpace(os.Getenv("APP_TIMEZONE"))
	if tzName == "" {
		tzName = strings.TrimSpace(os.Getenv("TZ"))
	}
	cfg.Timezone = tzName
	if tzName != "" {
		loc, err := time.LoadLocation(tzName)
		if err != nil {
			return cfg, fmt.Errorf("APP_TIMEZONE/TZ %q is invalid: %w (use IANA names e.g. Asia/Jakarta, Asia/Makassar, Asia/Jayapura)", tzName, err)
		}
		cfg.Location = loc
	} else {
		cfg.Location = time.Local
	}

	return cfg, nil
}

func (c Config) Development() bool {
	return c.Env == "development"
}

func (c Config) Standalone() bool {
	return c.MapperMode == ModeStandalone
}

func envOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
