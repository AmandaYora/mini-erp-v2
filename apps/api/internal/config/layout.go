package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrepareStorageLayout creates SessionDir and MediaDir and, once, moves files
// left in StorageDir by the old layout (everything was served from the storage
// root) into their new homes:
//
//   - whatsapp_session.db and its -wal/-shm/-journal companions -> SessionDir
//   - every other entry (product/, payment/, ...)              -> MediaDir
//
// It is idempotent and never overwrites: an entry that already exists at the
// destination stays where it is and is reported in skipped. It only acts when
// both directories are children of StorageDir, which is the default layout; a
// custom SESSION_DIR elsewhere is only created.
func PrepareStorageLayout(cfg *Config) (moved int, skipped []string, err error) {
	if err := os.MkdirAll(cfg.SessionDir, 0o700); err != nil {
		return 0, nil, fmt.Errorf("buat SESSION_DIR: %w", err)
	}
	if err := os.MkdirAll(cfg.MediaDir, 0o750); err != nil {
		return 0, nil, fmt.Errorf("buat folder media: %w", err)
	}
	root, err1 := filepath.Abs(cfg.StorageDir)
	sess, err2 := filepath.Abs(cfg.SessionDir)
	media, err3 := filepath.Abs(cfg.MediaDir)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, nil, fmt.Errorf("selesaikan path storage: %v %v %v", err1, err2, err3)
	}
	if filepath.Dir(sess) != root || filepath.Dir(media) != root {
		return 0, nil, nil // custom layout: nothing to migrate
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return 0, nil, err
	}
	for _, e := range entries {
		name := e.Name()
		src := filepath.Join(root, name)
		if src == sess || src == media || name == ".gitkeep" {
			continue
		}
		dstDir := media
		if strings.HasPrefix(name, "whatsapp_session.db") {
			dstDir = sess
		}
		dst := filepath.Join(dstDir, name)
		if _, err := os.Lstat(dst); err == nil {
			skipped = append(skipped, name)
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			return moved, skipped, fmt.Errorf("pindahkan %s: %w", name, err)
		}
		moved++
	}
	return moved, skipped, nil
}
