package application

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"mini-erp/internal/shared/apperror"
)

// Registered operational settings (F3b-7). Only settings some code actually
// reads belong here; the keys are the ones the reading modules use
// (stock: stock_adjust_*, delivery: delivery_require_evidence, print:
// document_profile). A test in internal/integration pins them to the
// readers' constants so a rename cannot silently orphan a setting.
const (
	SettingStockAlwaysApprove = "stock_adjust_always_approve"
	SettingStockApprovalMode  = "stock_adjust_approval_mode"
	SettingStockThreshold     = "stock_adjust_approval_threshold"
	SettingDeliveryEvidence   = "delivery_require_evidence"
	SettingDocumentProfile    = "document_profile"

	// Retention windows in days (F4-10). Read by the housekeeping sweep;
	// an RETENTION_*_DAYS environment variable still overrides them.
	SettingRetentionAudit       = "retention.audit_days"
	SettingRetentionSession     = "retention.session_days"
	SettingRetentionReservation = "retention.reservation_days"
	SettingRetentionAssistant   = "retention.assistant_days"
	SettingRetentionIdleThread  = "retention.idle_thread_days"
)

type settingKind int

const (
	kindBool settingKind = iota
	kindEnum
	kindNumber
	kindJSONObject
	kindDays
)

// SettingDef describes one registered setting.
type SettingDef struct {
	Key     string      `json:"key"`
	Kind    string      `json:"kind"`
	Default string      `json:"default"`
	Enum    []string    `json:"enum,omitempty"`
	Label   string      `json:"label"`
	kind    settingKind `json:"-"`
}

// SettingDefinitions lists the registered settings with their defaults, in
// display order. The defaults mirror what the readers assume when a key is
// unset.
func SettingDefinitions() []SettingDef {
	return []SettingDef{
		{Key: SettingStockAlwaysApprove, Kind: "boolean", Default: "true", kind: kindBool,
			Label: "Setiap koreksi stok wajib persetujuan penanggung jawab"},
		{Key: SettingStockApprovalMode, Kind: "enum", Default: "simple", Enum: []string{"simple", "strict"}, kind: kindEnum,
			Label: "Mode persetujuan koreksi stok (strict: penyetuju harus orang lain)"},
		{Key: SettingStockThreshold, Kind: "number", Default: "100", kind: kindNumber,
			Label: "Ambang koreksi stok (satuan dasar) yang perlu persetujuan bila tidak selalu wajib"},
		{Key: SettingDeliveryEvidence, Kind: "boolean", Default: "true", kind: kindBool,
			Label: "Bukti pengiriman wajib saat konfirmasi surat jalan"},
		{Key: SettingDocumentProfile, Kind: "json", Default: "", kind: kindJSONObject,
			Label: "Profil kop dokumen cetak (JSON)"},
		{Key: SettingRetentionAudit, Kind: "number", Default: "730", kind: kindDays,
			Label: "Retensi log audit (hari)"},
		{Key: SettingRetentionSession, Kind: "number", Default: "30", kind: kindDays,
			Label: "Retensi sesi login kedaluwarsa/dicabut (hari)"},
		{Key: SettingRetentionReservation, Kind: "number", Default: "7", kind: kindDays,
			Label: "Retensi reservasi stok tidak aktif (hari)"},
		{Key: SettingRetentionAssistant, Kind: "number", Default: "90", kind: kindDays,
			Label: "Retensi jejak asisten (hari)"},
		{Key: SettingRetentionIdleThread, Kind: "number", Default: "180", kind: kindDays,
			Label: "Retensi percakapan asisten tanpa pesan (hari)"},
	}
}

var settingKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,99}$`)

const maxSettingValue = 20000

func lookupDef(key string) (SettingDef, bool) {
	for _, d := range SettingDefinitions() {
		if d.Key == key {
			return d, true
		}
	}
	return SettingDef{}, false
}

// normalizeSettings validates and canonicalizes the submitted map. Registered
// keys are checked by kind (booleans become "true"/"false", numbers must be
// non-negative); unregistered keys are accepted with a key/size sanity limit
// so other modules can add settings without a company release.
func normalizeSettings(in map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(in))
	var fields []apperror.FieldError
	bad := func(key, msg string) {
		fields = append(fields, apperror.FieldError{Field: "settings." + key, Message: msg})
	}
	for rawKey, rawVal := range in {
		key := strings.TrimSpace(rawKey)
		val := strings.TrimSpace(rawVal)
		if !settingKeyRe.MatchString(key) {
			bad(rawKey, "nama pengaturan tidak valid")
			continue
		}
		if len(val) > maxSettingValue {
			bad(key, "terlalu panjang")
			continue
		}
		def, known := lookupDef(key)
		if !known {
			out[key] = val
			continue
		}
		switch def.kind {
		case kindBool:
			switch strings.ToLower(val) {
			case "true", "1":
				out[key] = "true"
			case "false", "0":
				out[key] = "false"
			default:
				bad(key, "harus true atau false")
			}
		case kindEnum:
			v := strings.ToLower(val)
			ok := false
			for _, e := range def.Enum {
				if e == v {
					ok = true
				}
			}
			if !ok {
				bad(key, "harus salah satu dari: "+strings.Join(def.Enum, ", "))
				continue
			}
			out[key] = v
		case kindNumber:
			f, err := strconv.ParseFloat(val, 64)
			if err != nil || f < 0 || f != f {
				bad(key, "harus angka tidak negatif")
				continue
			}
			out[key] = strconv.FormatFloat(f, 'f', -1, 64)
		case kindDays:
			n, err := strconv.Atoi(val)
			if err != nil || n < 1 || n > 3650 {
				bad(key, "harus bilangan bulat 1-3650 (hari)")
				continue
			}
			out[key] = strconv.Itoa(n)
		case kindJSONObject:
			if val == "" {
				out[key] = ""
				continue
			}
			var obj map[string]any
			if err := json.Unmarshal([]byte(val), &obj); err != nil || obj == nil {
				bad(key, "harus objek JSON yang valid")
				continue
			}
			out[key] = val
		}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation("", fields)
	}
	return out, nil
}

// withDefaults fills registered keys that were never saved.
func withDefaults(stored map[string]string) map[string]string {
	out := make(map[string]string, len(stored)+len(SettingDefinitions()))
	for k, v := range stored {
		out[k] = v
	}
	for _, d := range SettingDefinitions() {
		if _, ok := out[d.Key]; !ok && d.Default != "" {
			out[d.Key] = d.Default
		}
	}
	return out
}
