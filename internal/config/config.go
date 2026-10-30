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

	// Order poller (simrs mode only). Posts permintaan_lab to MedQLab saveOrder.
	OrderPoll         bool
	MedQLabBaseURL    string
	MedQLabXCons      string
	MedQLabXSign      string
	OrderPollSince    time.Time
	OrderPollInterval time.Duration
	OrderClaimTimeout time.Duration
	// OrderOnlyRM, when non-empty, limits the order poller to these no_rkm_medis values.
	OrderOnlyRM []string
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

	if err := loadOrderPoll(&cfg); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func loadOrderPoll(cfg *Config) error {
	cfg.OrderPoll = envBool("MEDQLAB_ORDER_POLL", false)
	cfg.MedQLabBaseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("MEDQLAB_BASE_URL")), "/")
	cfg.MedQLabXCons = strings.TrimSpace(os.Getenv("MEDQLAB_X_CONS"))
	cfg.MedQLabXSign = os.Getenv("MEDQLAB_X_SIGN")
	cfg.OrderPollInterval = envDuration("MEDQLAB_ORDER_INTERVAL", 5*time.Second)
	cfg.OrderClaimTimeout = envDuration("MEDQLAB_ORDER_CLAIM_TIMEOUT", 2*time.Minute)
	cfg.OrderOnlyRM = parseCSVList(os.Getenv("MEDQLAB_ORDER_ONLY_RM"))
	if !cfg.OrderPoll {
		return nil
	}
	if cfg.MapperMode != ModeSIMRS {
		return fmt.Errorf("MEDQLAB_ORDER_POLL requires MAPPER_MODE=%s", ModeSIMRS)
	}
	if cfg.MedQLabBaseURL == "" || cfg.MedQLabXCons == "" || strings.TrimSpace(cfg.MedQLabXSign) == "" {
		return fmt.Errorf("MEDQLAB_ORDER_POLL requires MEDQLAB_BASE_URL, MEDQLAB_X_CONS, and MEDQLAB_X_SIGN")
	}
	sinceRaw := strings.TrimSpace(os.Getenv("MEDQLAB_ORDER_SINCE"))
	if sinceRaw == "" {
		return fmt.Errorf("MEDQLAB_ORDER_POLL requires MEDQLAB_ORDER_SINCE (yyyy-MM-dd)")
	}
	since, err := time.ParseInLocation("2006-01-02", sinceRaw, cfg.Location)
	if err != nil {
		return fmt.Errorf("MEDQLAB_ORDER_SINCE must be yyyy-MM-dd: %w", err)
	}
	cfg.OrderPollSince = since
	return nil
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

func envBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}

func parseCSVList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func envDuration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}
