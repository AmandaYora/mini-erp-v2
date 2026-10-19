package integration

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"strings"
	"sync"
	"testing"

	mediacontracts "mini-erp/internal/modules/media/contracts"
)

func f3bJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y += 1 {
		for x := 0; x < w; x += 1 {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 92}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func f3bUpload(name string, raw []byte, owner int64) (*mediacontracts.MediaFile, error) {
	return testMods.media.Media().Upload(ctx(), mediacontracts.OwnerProduct, owner, mediacontracts.Upload{
		OriginalName: name, Content: bytes.NewReader(raw),
		Accept: []string{"image/jpeg", "image/png", "image/webp"},
	}, 0)
}

// F3b-3.6: optimisation, strict whitelist, primary promotion, thumbnails.
func TestF3bMediaOptimisationAndPrimary(t *testing.T) {
	freshDB(t)
	big := f3bJPEG(t, 2400, 1800)
	f1, err := f3bUpload("besar.jpg", big, 501)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if f1.Width > 1600 || f1.Height > 1600 || f1.Width != 1600 || !f1.IsPrimary {
		t.Fatalf("resized to fit 1600 and primary: %+v", f1)
	}
	if f1.OriginalSizeBytes != int64(len(big)) || f1.MIME != "image/jpeg" {
		t.Fatalf("metadata: %+v", f1)
	}
	var w, h int
	if err := testMods.db.QueryRow(`SELECT width, height FROM media_files WHERE id = ?`, f1.ID).Scan(&w, &h); err != nil || w != 1600 || h != 1200 {
		t.Fatalf("persisted dimensions: %d x %d %v", w, h, err)
	}

	f2, err := f3bUpload("kedua.jpg", f3bJPEG(t, 300, 200), 501)
	if err != nil || f2.IsPrimary {
		t.Fatalf("second file must not be primary: %+v %v", f2, err)
	}
	f3, _ := f3bUpload("ketiga.jpg", f3bJPEG(t, 120, 80), 501)

	// Thumbnails: one grouped read, primary only.
	urls, err := testMods.media.Media().PrimaryByOwners(ctx(), mediacontracts.OwnerProduct, []int64{501, 502})
	if err != nil || len(urls) != 1 || !strings.HasPrefix(urls[501], "/media/") {
		t.Fatalf("primary thumbnails: %+v %v", urls, err)
	}

	// Deleting the primary promotes the oldest remaining photo.
	if err := testMods.media.Media().Archive(ctx(), mediacontracts.OwnerProduct, 501, f1.ID, 0); err != nil {
		t.Fatal(err)
	}
	files, err := testMods.media.Media().List(ctx(), mediacontracts.OwnerProduct, 501)
	if err != nil || len(files) != 2 || files[0].ID != f2.ID || !files[0].IsPrimary || files[1].ID != f3.ID || files[1].IsPrimary {
		t.Fatalf("after deleting the primary: %+v %v", files, err)
	}
	// Deleting a non-primary keeps the primary.
	if err := testMods.media.Media().Archive(ctx(), mediacontracts.OwnerProduct, 501, f3.ID, 0); err != nil {
		t.Fatal(err)
	}
	files, _ = testMods.media.Media().List(ctx(), mediacontracts.OwnerProduct, 501)
	if len(files) != 1 || !files[0].IsPrimary {
		t.Fatalf("primary lost: %+v", files)
	}
	// Last photo gone: no primary to promote, no error.
	if err := testMods.media.Media().Archive(ctx(), mediacontracts.OwnerProduct, 501, f2.ID, 0); err != nil {
		t.Fatal(err)
	}
}

func TestF3bMediaWhitelist(t *testing.T) {
	freshDB(t)
	var g bytes.Buffer
	_ = gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.Black, color.White}), nil)
	cases := []struct {
		name string
		raw  []byte
	}{
		{"gif-sebagai.png", g.Bytes()},                  // content is a GIF
		{"dokumen.pdf", []byte("%PDF-1.4 isi pdf....")}, // the dead .pdf entry is gone
		{"gambar.gif", g.Bytes()},
		{"skrip.jpg", []byte("<?php echo 1; ?>")},
		{"kosong.png", nil},
	}
	for _, c := range cases {
		if _, err := f3bUpload(c.name, c.raw, 601); err == nil {
			t.Errorf("%s must be rejected", c.name)
		}
	}
	if n := f3bCount(t, `SELECT COUNT(*) FROM media_files WHERE owner_id = 601`); n != 0 {
		t.Fatalf("rejected uploads left %d rows", n)
	}
	// A JPEG whose extension lies is still a JPEG by content: accepted by the
	// sniffed type, and the stored key carries the real extension.
	f, err := f3bUpload("foto.png", f3bJPEG(t, 100, 100), 601)
	if err != nil || f.MIME != "image/jpeg" || !strings.HasSuffix(f.Key, ".jpg") {
		t.Fatalf("content decides the stored type: %+v %v", f, err)
	}
}

// Two uploads racing on an empty owner produce exactly one primary.
func TestF3bMediaFirstUploadRace(t *testing.T) {
	freshDB(t)
	raw := f3bJPEG(t, 64, 64)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f3bUpload("r.jpg", raw, 701)
		}()
	}
	wg.Wait()
	if n := f3bCount(t, `SELECT COUNT(*) FROM media_files WHERE owner_id = 701`); n != 5 {
		t.Fatalf("uploads stored = %d", n)
	}
	if n := f3bCount(t, `SELECT COUNT(*) FROM media_files WHERE owner_id = 701 AND is_primary = 1`); n != 1 {
		t.Fatalf("primary rows = %d, want exactly 1", n)
	}
}
