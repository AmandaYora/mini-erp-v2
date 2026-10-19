package application

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func jpegBytes(t *testing.T, w, h int, exif []byte) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	if exif == nil {
		return raw
	}
	// Splice an APP1 segment right after SOI.
	seg := append([]byte("Exif\x00\x00"), exif...)
	app1 := []byte{0xFF, 0xE1, byte((len(seg) + 2) >> 8), byte(len(seg) + 2)}
	app1 = append(app1, seg...)
	return append(append([]byte{0xFF, 0xD8}, app1...), raw[2:]...)
}

// exifOrientation builds a minimal little-endian TIFF block holding one
// Orientation tag.
func exifOrientation(o uint16) []byte {
	b := []byte("II*\x00")
	b = binary.LittleEndian.AppendUint32(b, 8)
	b = binary.LittleEndian.AppendUint16(b, 1)
	b = binary.LittleEndian.AppendUint16(b, 0x0112)
	b = binary.LittleEndian.AppendUint16(b, 3)
	b = binary.LittleEndian.AppendUint32(b, 1)
	b = binary.LittleEndian.AppendUint16(b, o)
	b = append(b, 0, 0)
	b = binary.LittleEndian.AppendUint32(b, 0)
	return b
}

func TestProcessImageShrinksLargePhoto(t *testing.T) {
	raw := jpegBytes(t, 4000, 3000, nil)
	out, err := processImage(raw, MaxDimProduct)
	if err != nil {
		t.Fatal(err)
	}
	if out.Width > 1600 || out.Height > 1600 || out.Width != 1600 || out.Height != 1200 {
		t.Fatalf("4000x3000 must fit inside 1600 keeping aspect: got %dx%d", out.Width, out.Height)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(out.Data))
	if err != nil || format != "jpeg" || cfg.Width != 1600 {
		t.Fatalf("output must be a decodable 1600px JPEG: %v %v %+v", err, format, cfg)
	}
	if len(out.Data) >= len(raw) {
		t.Errorf("optimised output (%d) should be smaller than the original (%d)", len(out.Data), len(raw))
	}
}

func TestProcessImageNeverEnlarges(t *testing.T) {
	out, err := processImage(jpegBytes(t, 640, 480, nil), MaxDimProduct)
	if err != nil || out.Width != 640 || out.Height != 480 {
		t.Fatalf("small image must keep its size: %+v %v", out, err)
	}
}

func TestProcessImageAppliesOrientationAndStripsExif(t *testing.T) {
	// Orientation 6 = rotate 90 clockwise: a 300x200 sensor image displays 200x300.
	raw := jpegBytes(t, 300, 200, exifOrientation(6))
	if got := jpegOrientation(raw); got != 6 {
		t.Fatalf("orientation read = %d", got)
	}
	out, err := processImage(raw, MaxDimProduct)
	if err != nil {
		t.Fatal(err)
	}
	if out.Width != 200 || out.Height != 300 {
		t.Fatalf("orientation not applied: %dx%d", out.Width, out.Height)
	}
	if jpegHasApp1(out.Data) {
		t.Fatal("EXIF must be dropped by the re-encode")
	}
	if got := jpegOrientation(out.Data); got != 1 {
		t.Fatalf("output orientation = %d", got)
	}
}

func TestProcessImageKeepsTransparencyAsPNG(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2000, 10))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 128})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	out, err := processImage(buf.Bytes(), MaxDimProduct)
	if err != nil || out.MIME != "image/png" || out.Width != 1600 {
		t.Fatalf("transparent image must stay PNG and shrink: %+v %v", out, err)
	}
}

func TestProcessImagePassesThroughSmallCleanFile(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	out, err := processImage(buf.Bytes(), MaxDimProduct)
	if err != nil || !bytes.Equal(out.Data, buf.Bytes()) || out.MIME != "image/png" {
		t.Fatalf("tiny clean PNG is stored as is: %v", err)
	}
}

func TestProcessImageRejects(t *testing.T) {
	// A GIF is not an accepted format even when the name says .png.
	var g bytes.Buffer
	_ = gif.Encode(&g, image.NewPaletted(image.Rect(0, 0, 4, 4), color.Palette{color.Black, color.White}), nil)
	if _, err := processImage(g.Bytes(), MaxDimProduct); err == nil {
		t.Error("GIF must be rejected")
	}
	if _, err := processImage([]byte("%PDF-1.4 not an image"), MaxDimProduct); err == nil {
		t.Error("PDF must be rejected")
	}
	// Decompression bomb: a tiny PNG whose header declares 20000x20000.
	var p bytes.Buffer
	_ = png.Encode(&p, image.NewNRGBA(image.Rect(0, 0, 1, 1)))
	raw := append([]byte{}, p.Bytes()...)
	binary.BigEndian.PutUint32(raw[16:20], 20000)
	binary.BigEndian.PutUint32(raw[20:24], 20000)
	if _, err := processImage(raw, MaxDimProduct); err == nil {
		t.Error("48+ megapixel declaration must be rejected before decoding")
	}
}
