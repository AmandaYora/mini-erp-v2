package application

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder (decode only)
)

// Image optimisation limits (F3b-3.6).
const (
	// MaxPixels bounds the decoded size: a tiny PNG can declare gigapixels
	// (decompression bomb), so the header is checked before the full decode.
	MaxPixels = 48_000_000
	// MaxDimProduct / MaxDimProof are the fit-inside box of resized output.
	MaxDimProduct = 1600
	MaxDimProof   = 2400
	jpegQuality   = 85
	// passThroughBytes: an already small, already upright image without
	// metadata is stored untouched; re-encoding it could only make it worse.
	passThroughBytes = 300 << 10
)

// imageSlots bounds concurrent image work: decoding a 5 MB photo is CPU and
// memory heavy, and a bulk upload must not take the container down.
var imageSlots = make(chan struct{}, 2)

// ErrUnsupportedImage is returned for anything that is not a JPEG, PNG or
// WebP picture of sane size.
var ErrUnsupportedImage = errors.New("hanya gambar JPG, PNG, atau WebP yang didukung")

// processedImage is the stored form of an upload.
type processedImage struct {
	Data   []byte
	MIME   string
	Ext    string
	Width  int
	Height int
}

// processImage validates and optimises an upload: decode (JPEG/PNG/WebP
// only), apply the EXIF orientation, shrink to fit maxDim without enlarging,
// and re-encode as JPEG q85 (PNG when the picture has transparency). The
// re-encode drops every metadata block, EXIF GPS included.
func processImage(raw []byte, maxDim int) (*processedImage, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrUnsupportedImage
	}
	if format != "jpeg" && format != "png" && format != "webp" {
		return nil, ErrUnsupportedImage
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > MaxPixels {
		return nil, errors.New("dimensi gambar terlalu besar")
	}
	orientation := 1
	if format == "jpeg" {
		orientation = jpegOrientation(raw)
	}
	if cfg.Width <= maxDim && cfg.Height <= maxDim && orientation == 1 &&
		len(raw) <= passThroughBytes && !hasEXIF(raw, format) {
		ext, mime := ".jpg", "image/jpeg"
		switch format {
		case "png":
			ext, mime = ".png", "image/png"
		case "webp":
			ext, mime = ".webp", "image/webp"
		}
		return &processedImage{Data: raw, MIME: mime, Ext: ext, Width: cfg.Width, Height: cfg.Height}, nil
	}

	imageSlots <- struct{}{}
	defer func() { <-imageSlots }()

	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, ErrUnsupportedImage
	}
	img = applyOrientation(img, orientation)
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > maxDim || h > maxDim {
		scale := float64(maxDim) / float64(max(w, h))
		nw, nh := max(1, int(float64(w)*scale+0.5)), max(1, int(float64(h)*scale+0.5))
		dst := image.NewNRGBA(image.Rect(0, 0, nw, nh))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
		img, w, h = dst, nw, nh
	}
	var buf bytes.Buffer
	if hasAlpha(img) {
		if err := png.Encode(&buf, img); err != nil {
			return nil, err
		}
		return &processedImage{Data: buf.Bytes(), MIME: "image/png", Ext: ".png", Width: w, Height: h}, nil
	}
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return &processedImage{Data: buf.Bytes(), MIME: "image/jpeg", Ext: ".jpg", Width: w, Height: h}, nil
}

// hasAlpha reports whether any pixel is not fully opaque.
func hasAlpha(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return !o.Opaque()
	}
	return false // formats without alpha (YCbCr, Gray) are opaque
}

// hasEXIF reports an EXIF/XMP metadata block that a pass-through would keep.
func hasEXIF(raw []byte, format string) bool {
	switch format {
	case "jpeg":
		return jpegHasApp1(raw)
	case "png":
		return bytes.Contains(raw, []byte("eXIf")) || bytes.Contains(raw, []byte("iTXt"))
	case "webp":
		return bytes.Contains(raw, []byte("EXIF")) || bytes.Contains(raw, []byte("XMP "))
	}
	return false
}

func jpegHasApp1(raw []byte) bool {
	_, ok := jpegApp1(raw)
	return ok
}

// jpegApp1 returns the first APP1 "Exif" payload (the TIFF block).
func jpegApp1(raw []byte) ([]byte, bool) {
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return nil, false
	}
	i := 2
	for i+4 <= len(raw) {
		if raw[i] != 0xFF {
			return nil, false
		}
		marker := raw[i+1]
		if marker == 0xD8 || (marker >= 0xD0 && marker <= 0xD7) || marker == 0x01 {
			i += 2
			continue
		}
		if marker == 0xDA || marker == 0xD9 { // start of scan / end: no more headers
			return nil, false
		}
		size := int(binary.BigEndian.Uint16(raw[i+2 : i+4]))
		if size < 2 || i+2+size > len(raw) {
			return nil, false
		}
		seg := raw[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) >= 6 && string(seg[:6]) == "Exif\x00\x00" {
			return seg[6:], true
		}
		i += 2 + size
	}
	return nil, false
}

// jpegOrientation reads the EXIF orientation tag (1 when absent/unreadable).
func jpegOrientation(raw []byte) int {
	tiff, ok := jpegApp1(raw)
	if !ok || len(tiff) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(tiff[4:8]))
	if off < 8 || off+2 > len(tiff) {
		return 1
	}
	n := int(bo.Uint16(tiff[off : off+2]))
	for k := 0; k < n; k++ {
		e := off + 2 + k*12
		if e+12 > len(tiff) {
			return 1
		}
		if bo.Uint16(tiff[e:e+2]) == 0x0112 { // Orientation
			v := int(bo.Uint16(tiff[e+8 : e+10]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation rotates/flips img so it displays upright (EXIF 1-8).
func applyOrientation(img image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	swap := o >= 5
	var dst *image.NRGBA
	if swap {
		dst = image.NewNRGBA(image.Rect(0, 0, h, w))
	} else {
		dst = image.NewNRGBA(image.Rect(0, 0, w, h))
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
