package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every environment-driven setting. No secrets live in code;
// copy .env.example to .env locally, inject real values in production.
type Config struct {
	Env         string
	Port        string
	PublicDir   string
	StorageDir  string
	Storage     Storage
	DBDSN       string
	JWTSecret   string
	AccessTTL   time.Duration
	RefreshTTL  time.Duration
	CORSOrigins []string
}

// Storage mirrors the legacy object-storage settings (IDCloudHost S3):
// same endpoint/bucket/key layout, env-driven prefixes. Driver "local"
// keeps bytes under StorageDir (dev default); "s3" requires bucket + keys.
type Storage struct {
	Driver           string
	Endpoint         string
	Bucket           string
	Region           string
	ForcePathStyle   bool
	AccessKeyID      string
	SecretAccessKey  string
	PublicBaseURL    string
	SignedURLTTLSecs int
	ProductPrefix    string
	TransferPrefix   string
	DeliveryPrefix   string
}

// parseDuration extends time.ParseDuration with day/week suffixes, because
// JWT lifetimes are configured as "15m"/"7d". The standard parser rejects
// "7d" — silently falling back to a default there caused session/token
// lifetime drift in the legacy system (KI-06 class), so unknown units are a
// hard error here, never a silent default.
func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	mult := map[byte]time.Duration{'d': 24 * time.Hour, 'w': 7 * 24 * time.Hour}
	if len(s) < 2 {
		return 0, fmt.Errorf("bad duration %q", s)
	}
	unit := s[len(s)-1]
	m, ok := mult[unit]
	if !ok {
		return 0, fmt.Errorf("bad duration %q", s)
	}
	n, err := strconv.Atoi(s[:len(s)-1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("bad duration %q", s)
	}
	return time.Duration(n) * m, nil
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Load reads ../../.env (repo root, API runs from apps/api) then a local
// .env fallback. Missing files are fine — containers inject env directly.
func Load() (*Config, error) {
	_ = godotenv.Load("../../.env")
	_ = godotenv.Load(".env")

	accessTTL, err := parseDuration(get("JWT_ACCESS_EXPIRES_IN", "15m"))
	if err != nil {
		return nil, fmt.Errorf("JWT_ACCESS_EXPIRES_IN: %w", err)
	}
	refreshTTL, err := parseDuration(get("JWT_REFRESH_EXPIRES_IN", "7d"))
	if err != nil {
		return nil, fmt.Errorf("JWT_REFRESH_EXPIRES_IN: %w", err)
	}

	cfg := &Config{
		Env:         get("APP_ENV", "development"),
		Port:        get("APP_PORT", "8080"),
		PublicDir:   get("PUBLIC_DIR", ""),
		StorageDir:  get("STORAGE_DIR", "./storage"),
		Storage:     loadStorage(get("APP_ENV", "development")),
		DBDSN:       get("DB_DSN", ""),
		JWTSecret:   get("JWT_SECRET", ""),
		AccessTTL:   accessTTL,
		RefreshTTL:  refreshTTL,
		CORSOrigins: splitOrigins(get("CORS_ALLOWED_ORIGINS", "")),
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	// Example/weak secrets are rejected everywhere except development: a
	// non-production deploy with APP_ENV unset or mistyped must not run on
	// the published example value or a short secret. Development keeps the
	// convenient default (A5).
	if cfg.Env != "development" {
		if cfg.JWTSecret == "change-me" {
			return nil, fmt.Errorf("JWT_SECRET must not be the example value outside development")
		}
		if len(cfg.JWTSecret) < 32 {
			return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters outside development")
		}
	}
	if cfg.Storage.Driver == "s3" && (cfg.Storage.Bucket == "" || cfg.Storage.AccessKeyID == "" || cfg.Storage.SecretAccessKey == "") {
		return nil, fmt.Errorf("STORAGE_BUCKET, STORAGE_ACCESS_KEY_ID, dan STORAGE_SECRET_ACCESS_KEY wajib diisi untuk STORAGE_DRIVER=s3")
	}
	return cfg, nil
}

// loadStorage reads object-storage settings with legacy-matching defaults:
// IDCloudHost endpoint/bucket, prefixes derived as {root}/{env}/upload/…
// (legacy separates environments in the prefix itself).
func loadStorage(appEnv string) Storage {
	root := cleanPrefix(get("STORAGE_ROOT_PREFIX", "vioni"))
	uploadRoot := cleanPrefix(get("STORAGE_UPLOAD_ROOT_PREFIX", ""))
	if uploadRoot == "" {
		uploadRoot = joinKey(root, appEnv, "upload")
	}
	ttl, err := strconv.Atoi(get("STORAGE_SIGNED_URL_TTL_SECONDS", "300"))
	if err != nil || ttl <= 0 {
		ttl = 300
	}
	pathStyle := true
	if v := strings.TrimSpace(get("STORAGE_FORCE_PATH_STYLE", "true")); v != "" {
		pathStyle = v == "true" || v == "1"
	}
	productPrefix := cleanPrefix(get("STORAGE_PRODUCT_PREFIX", ""))
	if productPrefix == "" {
		productPrefix = joinKey(uploadRoot, "product")
	}
	transferPrefix := cleanPrefix(get("STORAGE_PAYMENT_TRANSFER_PREFIX", ""))
	if transferPrefix == "" {
		transferPrefix = joinKey(uploadRoot, "order", "transfer")
	}
	deliveryPrefix := cleanPrefix(get("STORAGE_DELIVERY_PROOF_PREFIX", ""))
	if deliveryPrefix == "" {
		deliveryPrefix = joinKey(uploadRoot, "order", "delivery-proof")
	}
	return Storage{
		Driver:           get("STORAGE_DRIVER", "local"),
		Endpoint:         get("STORAGE_ENDPOINT", "https://is3.cloudhost.id"),
		Bucket:           get("STORAGE_BUCKET", "mini-erp"),
		Region:           get("STORAGE_REGION", "auto"),
		ForcePathStyle:   pathStyle,
		AccessKeyID:      get("STORAGE_ACCESS_KEY_ID", ""),
		SecretAccessKey:  get("STORAGE_SECRET_ACCESS_KEY", ""),
		PublicBaseURL:    strings.TrimRight(get("STORAGE_PUBLIC_BASE_URL", ""), "/"),
		SignedURLTTLSecs: ttl,
		ProductPrefix:    productPrefix,
		TransferPrefix:   transferPrefix,
		DeliveryPrefix:   deliveryPrefix,
	}
}

// cleanPrefix trims slashes and whitespace (legacy parity).
func cleanPrefix(s string) string {
	return strings.Trim(strings.TrimSpace(s), "/")
}

// joinKey joins key segments with single slashes, skipping empties.
func joinKey(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = cleanPrefix(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, "/")
}

func splitOrigins(s string) []string {
	var out []string
	for _, o := range strings.Split(s, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}
