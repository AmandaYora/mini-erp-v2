package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"mini-erp/internal/shared/signedurl"
)

// Config holds every environment-driven setting. No secrets live in code;
// copy .env.example to .env locally, inject real values in production.
type Config struct {
	Env        string
	Port       string
	PublicDir  string
	StorageDir string
	// SessionDir holds secrets that must never be served over HTTP (the
	// WhatsApp session). MediaDir holds user uploads and is the only folder the
	// API serves, through signed URLs. Neither may contain the other.
	SessionDir string
	MediaDir   string
	// MediaSigningKey signs local media URLs (see internal/shared/signedurl).
	MediaSigningKey []byte
	Storage         Storage
	DBDSN           string
	JWTSecret       string
	AccessTTL       time.Duration
	RefreshTTL      time.Duration
	CORSOrigins     []string
	// TrustedProxies lists CIDR/IP of proxies whose forwarding headers may be
	// honoured when resolving the client IP (F1-3). Empty = trust none.
	TrustedProxies string
	ClientIPHeader string
	// DBPool bounds the database connection pool (F6-4).
	DBMaxOpen  int
	DBMaxIdle  int
	DBLife     time.Duration
	DBIdleTime time.Duration
	// MetricsToken guards /api/v1/metrics; empty disables the endpoint (F6-5).
	MetricsToken string
	// ShutdownTimeout bounds graceful shutdown (F6-6).
	ShutdownTimeout time.Duration
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
	// UploadRoot is the deployment's own upload prefix (F1-9). Reads of legacy
	// keys keep working (presigned by key), but new uploads and deletes must
	// stay inside this prefix.
	UploadRoot string
	// ProtectedPrefixes are never deleted by Archive (F1-9), e.g. the legacy
	// production prefix still serving the old system during rollback window.
	ProtectedPrefixes []string
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

	// APP_ENV has no default on purpose. A forgotten variable used to fall back
	// to "development", which is the one environment where weak secrets are
	// accepted — so a misconfigured production deploy silently ran on them.
	env := strings.TrimSpace(get("APP_ENV", ""))
	switch env {
	case "development", "test", "staging", "production":
	default:
		return nil, fmt.Errorf("APP_ENV wajib diisi: development | test | staging | production (nilai saat ini %q)", env)
	}

	cfg := &Config{
		Env:             env,
		Port:            get("APP_PORT", "8080"),
		PublicDir:       get("PUBLIC_DIR", ""),
		StorageDir:      get("STORAGE_DIR", "./storage"),
		Storage:         loadStorage(env),
		DBDSN:           get("DB_DSN", ""),
		JWTSecret:       get("JWT_SECRET", ""),
		AccessTTL:       accessTTL,
		RefreshTTL:      refreshTTL,
		CORSOrigins:     splitOrigins(get("CORS_ALLOWED_ORIGINS", "")),
		TrustedProxies:  strings.TrimSpace(get("TRUSTED_PROXIES", "")),
		ClientIPHeader:  strings.TrimSpace(get("CLIENT_IP_HEADER", "X-Forwarded-For")),
		DBMaxOpen:       getInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdle:       getInt("DB_MAX_IDLE_CONNS", 10),
		DBLife:          getDur("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		DBIdleTime:      getDur("DB_CONN_MAX_IDLETIME", 2*time.Minute),
		MetricsToken:    get("METRICS_TOKEN", ""),
		ShutdownTimeout: getDur("SHUTDOWN_TIMEOUT", 25*time.Second),
	}
	if cfg.ClientIPHeader == "" {
		cfg.ClientIPHeader = "X-Forwarded-For"
	}
	if cfg.DBMaxIdle > cfg.DBMaxOpen {
		return nil, fmt.Errorf("DB_MAX_IDLE_CONNS tidak boleh lebih besar dari DB_MAX_OPEN_CONNS")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	// Example/weak secrets are rejected everywhere except development: a
	// non-production deploy with APP_ENV unset or mistyped must not run on
	// the published example value or a short secret. Development keeps the
	// convenient default (A5).
	if cfg.Env != "development" && cfg.Env != "test" {
		if cfg.JWTSecret == "change-me" {
			return nil, fmt.Errorf("JWT_SECRET must not be the example value outside development")
		}
		if len(cfg.JWTSecret) < 32 {
			return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters outside development")
		}
		if strings.Count(cfg.JWTSecret, string(cfg.JWTSecret[0])) == len(cfg.JWTSecret) {
			return nil, fmt.Errorf("JWT_SECRET must not be one repeated character")
		}
	}
	for _, o := range cfg.CORSOrigins {
		if o == "*" {
			return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS tidak boleh '*': sebutkan origin secara eksplisit")
		}
	}
	if cfg.Storage.Driver == "s3" && (cfg.Storage.Bucket == "" || cfg.Storage.AccessKeyID == "" || cfg.Storage.SecretAccessKey == "") {
		return nil, fmt.Errorf("STORAGE_BUCKET, STORAGE_ACCESS_KEY_ID, dan STORAGE_SECRET_ACCESS_KEY wajib diisi untuk STORAGE_DRIVER=s3")
	}
	cfg.SessionDir = get("SESSION_DIR", filepath.Join(cfg.StorageDir, "_private"))
	cfg.MediaDir = filepath.Join(cfg.StorageDir, "media")
	if err := checkDirsDisjoint(cfg.SessionDir, cfg.MediaDir); err != nil {
		return nil, err
	}
	// A dedicated MEDIA_SIGNING_KEY is preferred in production; otherwise the
	// key is derived from JWT_SECRET so no extra mandatory secret is introduced.
	if k := get("MEDIA_SIGNING_KEY", ""); k != "" {
		cfg.MediaSigningKey = []byte(k)
	} else {
		cfg.MediaSigningKey = signedurl.Derive(cfg.JWTSecret, "media-url-v1")
	}
	// Production rules run last on purpose: checkProduction inspects the
	// derived MediaSigningKey (must differ from JWT_SECRET), so calling it
	// before the derivation above would compare against a nil key and let a
	// MEDIA_SIGNING_KEY equal to JWT_SECRET slip through.
	if cfg.Env == "production" {
		if err := checkProduction(cfg); err != nil {
			return nil, err
		}
	}
	return cfg, nil
}

// checkProduction holds the rules that only make sense for a live deployment.
// They are errors, not warnings: a production start must fail loudly rather
// than run on development conveniences.
func checkProduction(cfg *Config) error {
	if cfg.DBDSN == "" {
		return fmt.Errorf("DB_DSN wajib diisi di production")
	}
	if strings.HasPrefix(cfg.DBDSN, "root:@") || strings.HasPrefix(cfg.DBDSN, "root@") {
		return fmt.Errorf("DB_DSN production tidak boleh memakai root tanpa password")
	}
	if len(cfg.CORSOrigins) == 0 {
		return fmt.Errorf("CORS_ALLOWED_ORIGINS wajib diisi di production")
	}
	if os.Getenv("MEDIA_SIGNING_KEY") == "" {
		return fmt.Errorf("MEDIA_SIGNING_KEY wajib diisi di production (kunci terpisah dari JWT_SECRET)")
	}
	if string(cfg.MediaSigningKey) == cfg.JWTSecret {
		return fmt.Errorf("MEDIA_SIGNING_KEY tidak boleh sama dengan JWT_SECRET")
	}
	if cfg.Storage.Driver == "local" && !filepath.IsAbs(cfg.StorageDir) {
		return fmt.Errorf("STORAGE_DIR production harus path absolut (nilai saat ini %q)", cfg.StorageDir)
	}
	if cfg.Storage.Driver == "s3" && os.Getenv("STORAGE_UPLOAD_ROOT_PREFIX") == "" {
		return fmt.Errorf("STORAGE_UPLOAD_ROOT_PREFIX wajib diisi eksplisit di production dengan S3; prefix turunan otomatis bisa menimpa berkas sistem lama di bucket yang sama")
	}
	for _, p := range cfg.Storage.ProtectedPrefixes {
		if p != "" && (p == cfg.Storage.UploadRoot || strings.HasPrefix(cfg.Storage.UploadRoot, p+"/") || strings.HasPrefix(p, cfg.Storage.UploadRoot+"/")) {
			return fmt.Errorf("STORAGE_UPLOAD_ROOT_PREFIX (%q) tidak boleh sama/bertumpuk dengan STORAGE_PROTECTED_PREFIXES (%q): unggahan baru akan bercampur dengan berkas sistem lama", cfg.Storage.UploadRoot, p)
		}
	}
	return nil
}

// checkDirsDisjoint refuses a layout where one directory contains the other:
// serving MediaDir must never expose SessionDir.
func checkDirsDisjoint(session, media string) error {
	s, err1 := filepath.Abs(session)
	m, err2 := filepath.Abs(media)
	if err1 != nil || err2 != nil {
		return fmt.Errorf("SESSION_DIR/STORAGE_DIR tidak dapat diselesaikan: %v %v", err1, err2)
	}
	inside := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
	}
	if inside(m, s) || inside(s, m) {
		return fmt.Errorf("SESSION_DIR (%s) dan folder media (%s) tidak boleh saling berada di dalam; sesi WhatsApp akan ikut tersaji", s, m)
	}
	return nil
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
		UploadRoot:       uploadRoot,
		ProtectedPrefixes: func() []string {
			var out []string
			for _, p := range strings.Split(get("STORAGE_PROTECTED_PREFIXES", ""), ",") {
				if p = cleanPrefix(p); p != "" {
					out = append(out, p)
				}
			}
			return out
		}(),
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

func getInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func getDur(key string, def time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := parseDuration(v); err == nil {
			return d
		}
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
