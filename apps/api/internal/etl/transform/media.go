package transform

import (
	"crypto/sha1"
	"encoding/hex"
	"path"
	"strings"

	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/row"
)

var mediaExt = map[string]string{".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".pdf": "application/pdf"}

// media prepares candidate media rows (design F5-9). The files themselves are
// checked and copied by the media phase, which then writes the final
// media_files rows; a candidate whose file is missing is NOT loaded.
func (s *state) media() error {
	add := func(ownerType string, owner int64, legacyKey, name, mime string, size int64, primary bool, legacyID int64, purpose string, checksum string) {
		ext := strings.ToLower(path.Ext(legacyKey))
		if _, ok := mediaExt[ext]; !ok {
			s.find(report.Warn, "MEDIA_EXT", "media_files", legacyID, "ekstensi %q di luar daftar putih: tidak dimigrasi (diarsipkan)", ext)
			return
		}
		key := legacyKey
		if len(key) > 255 {
			sum := sha1.Sum([]byte(legacyKey))
			key = "media/" + hex.EncodeToString(sum[:]) + ext
			s.find(report.Warn, "MEDIA_KEY_LONG", "media_files", legacyID, "kunci berkas >255 karakter diganti hash")
		}
		switch purpose {
		case "delivery_location_photo":
			name = "[foto-lokasi] " + name
		case "delivery_signed_document":
			name = "[dokumen-ttd] " + name
		}
		if name == "" {
			name = path.Base(legacyKey)
		}
		s.emit("media_candidates", row.Row{
			"owner_type": ownerType, "owner_id": owner, "src_key": legacyKey, "file_key": key,
			"original_name": s.trunc("media_files", legacyID, "original_name", name, 255), "mime": mime, "size_bytes": size,
			"is_primary": b2i(primary), "origin": "legacy", "legacy_id": legacyID, "legacy_checksum": checksum,
		})
	}
	for _, m := range sortedBy(s.raw["media_files"], "id_media_file") {
		lid := m.Int("id_media_file")
		if m.Has("archived_at") {
			continue
		}
		var owner int64
		var ownerType string
		var ok bool
		switch m.Str("owner_type") {
		case "product":
			ownerType = "product"
			owner, ok = s.id("products", m.Int("owner_id"))
		case "delivery_note":
			// DL-01: the legacy purpose decides the revamp owner type. The signed
			// SJ scan/photo is its own kind (it gates confirmation); a location
			// photo - or any unknown/empty purpose, which legacy never produced
			// (parsePurpose rejected it) - stays the optional photo.
			ownerType = "delivery_proof"
			if m.Str("purpose") == "delivery_signed_document" {
				ownerType = "delivery_document"
			}
			owner, ok = s.dnOfLegacy[m.Int("owner_id")], s.dnOfLegacy[m.Int("owner_id")] != 0
		case "payment":
			ownerType = "payment_proof"
			owner, ok = s.id("payments", m.Int("owner_id"))
		default:
			s.find(report.Info, "MEDIA_OWNER", "media_files", lid, "pemilik %q tidak dimigrasi (arsip)", m.Str("owner_type"))
			continue
		}
		if !ok {
			s.find(report.Blocker, "MEDIA_OWNER_ORPHAN", "media_files", lid, "pemilik media %s/%d tidak termigrasi", m.Str("owner_type"), m.Int("owner_id"))
			continue
		}
		mime := m.Str("mime_type")
		if len(mime) > 100 {
			mime = mime[:100]
		}
		add(ownerType, owner, m.Str("object_key"), m.Str("original_name"), mime, m.Int("size_bytes"), m.Bool("is_primary"), lid, m.Str("purpose"), m.Str("checksum"))
	}
	for _, p := range s.proofs {
		ext := strings.ToLower(path.Ext(p.key))
		add("payment_proof", p.newPayment, p.key, path.Base(p.key), mediaExt[ext], 0, true, -p.legacy, "payment_proof", "")
	}
	return nil
}
