package application

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	qrcode "github.com/skip2/go-qrcode"
	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// QR label (J2): PNG per operational target, payload chosen for the scan
// loop — variant barcode first (wedge/camera resolve it via
// stock/scan-product exactly), else the raw code (resolves via product
// search, legacy F-08.2). Reuses the go-qrcode already vendored for WA
// pairing (no new dependency).
//
// Size clamps to 128–1024 px (default 256): labels stay scannable without
// serving megabyte PNGs by accident.

const (
	QRDefaultSize = 256
	QRMinSize     = 128
	QRMaxSize     = 1024
	// Bulk export streams the ZIP entry by entry (P3): heap stays O(one
	// label) no matter how big the catalog is.
	QRExportFileName = "Produk-QR-Codes.zip"
	// QRMatrixSize is the pixel width encoded into bulk-export PNGs.
	QRMatrixSize = 480
)

// QRLabel returns one product's QR PNG plus its payload. variantID selects
// one variant's target (0 = the product-level default rule below).
func (s *Service) QRLabel(ctx context.Context, id, variantID int64, size int) ([]byte, string, error) {
	p, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if p == nil {
		return nil, "", apperror.NotFound("Produk")
	}
	payload, err := selectQRPayload(p, variantID)
	if err != nil {
		return nil, "", err
	}
	png, err := qrcode.Encode(payload, qrcode.Medium, clampQRSize(size))
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	return png, payload, nil
}

// selectQRPayload picks the encoded value for one label. Pure: unit-tested
// without a database.
func selectQRPayload(p *contracts.Product, variantID int64) (string, error) {
	if variantID != 0 {
		for _, v := range p.Variants {
			if v.ID == variantID {
				if code := strings.TrimSpace(v.Barcode); code != "" {
					return code, nil
				}
				if code := strings.TrimSpace(v.Code); code != "" {
					return code, nil
				}
				return "", apperror.Conflict("Varian tidak punya kode maupun barcode untuk QR")
			}
		}
		return "", apperror.NotFound("Varian")
	}
	payload := strings.TrimSpace(p.Code)
	firstBarcode := ""
	for _, v := range p.Variants {
		if code := strings.TrimSpace(v.Barcode); code != "" {
			if firstBarcode == "" {
				firstBarcode = code
			}
			if v.IsDefault {
				payload = code
				firstBarcode = ""
				break
			}
		}
	}
	if firstBarcode != "" {
		payload = firstBarcode
	}
	if payload == "" {
		return "", apperror.Conflict("Produk tidak punya kode maupun barcode untuk QR")
	}
	return payload, nil
}

func clampQRSize(size int) int {
	if size < QRMinSize {
		return QRDefaultSize
	}
	if size > QRMaxSize {
		return QRMaxSize
	}
	return size
}

// QRTarget is one printable label: variant-level when the product sells by
// variant (name "{product} - {variant}", legacy F-08.1), else product-level.
// Category routes the file into its ZIP folder.
type QRTarget struct {
	ProductID int64
	VariantID int64
	Code      string
	Name      string
	Category  string
	Payload   string
}

// assembleQRTargets folds flat catalog rows into operational targets. Pure:
// unit-tested without a database.
func assembleQRTargets(rows []infrastructure.QRSource) []*QRTarget {
	byProduct := map[int64][]infrastructure.QRSource{}
	order := []int64{}
	for _, r := range rows {
		if _, ok := byProduct[r.ProductID]; !ok {
			order = append(order, r.ProductID)
		}
		byProduct[r.ProductID] = append(byProduct[r.ProductID], r)
	}
	out := []*QRTarget{}
	for _, pid := range order {
		group := byProduct[pid]
		head := group[0]
		withVariants := false
		for _, r := range group {
			if r.VariantID != 0 {
				withVariants = true
				payload := strings.TrimSpace(r.Barcode)
				if payload == "" {
					payload = strings.TrimSpace(r.VariantCode)
				}
				if payload == "" {
					continue
				}
				out = append(out, &QRTarget{
					ProductID: pid, VariantID: r.VariantID,
					Code: r.VariantCode, Name: head.ProductName + " - " + r.VariantName,
					Category: head.Category, Payload: payload,
				})
			}
		}
		if withVariants {
			continue
		}
		payload := strings.TrimSpace(head.ProductCode)
		if payload == "" {
			continue
		}
		out = append(out, &QRTarget{
			ProductID: pid, Code: head.ProductCode, Name: head.ProductName,
			Category: head.Category, Payload: payload,
		})
	}
	if out == nil {
		out = []*QRTarget{}
	}
	return out
}

// sanitizeQRName replaces Windows-illegal filename characters and caps
// length in runes (legacy F-08.4). Pure: unit-tested.
func sanitizeQRName(s string, maxRunes int) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			b.WriteRune('-')
		default:
			if r < 0x20 {
				b.WriteRune('-')
			} else {
				b.WriteRune(r)
			}
		}
	}
	out := strings.Trim(b.String(), " .")
	if utf8.RuneCountInString(out) > maxRunes {
		runes := []rune(out)
		out = string(runes[:maxRunes])
		out = strings.Trim(out, " .")
	}
	if out == "" {
		out = "tanpa-nama"
	}
	return out
}

// qrFileName names one ZIP entry `{code}_{name}.png`, deduplicating with a
// `_2`, `_3`, … suffix (legacy F-08.4). Pure: unit-tested.
func qrFileName(code, name string, seen map[string]int) string {
	base := sanitizeQRName(code, 50) + "_" + sanitizeQRName(name, 50) + ".png"
	n, ok := seen[base]
	if !ok {
		seen[base] = 1
		return base
	}
	seen[base] = n + 1
	return strings.TrimSuffix(base, ".png") + fmt.Sprintf("_%d.png", n+1)
}

// QRTargets resolves every operational label target in one grouped catalog
// read (P3). Empty catalog is a clean failure, reported before any download
// headers are sent.
func (s *Service) QRTargets(ctx context.Context) ([]*QRTarget, error) {
	rows, err := s.repo.QRSources(ctx)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	targets := assembleQRTargets(rows)
	if len(targets) == 0 {
		return nil, apperror.Conflict("Tidak ada produk dengan kode yang valid untuk diekspor.")
	}
	return targets, nil
}

// QRZip streams targets into a ZIP on w, foldered per category ("Tanpa
// Kategori" when uncategorized). Entry-by-entry streaming keeps heap O(one
// label) no matter how big the catalog is (P3). Mid-stream I/O failures are
// unreportable by nature — same contract as the other binary downloads.
func QRZip(targets []*QRTarget, w io.Writer) error {
	zw := zip.NewWriter(w)
	seen := map[string]map[string]int{}
	for _, t := range targets {
		folder := strings.TrimSpace(t.Category)
		if folder == "" {
			folder = "Tanpa Kategori"
		}
		if seen[folder] == nil {
			seen[folder] = map[string]int{}
		}
		png, err := qrcode.Encode(t.Payload, qrcode.Medium, QRMatrixSize)
		if err != nil {
			_ = zw.Close()
			return apperror.Internal(err)
		}
		f, err := zw.Create(folder + "/" + qrFileName(t.Code, t.Name, seen[folder]))
		if err != nil {
			_ = zw.Close()
			return apperror.Internal(err)
		}
		if _, err := f.Write(png); err != nil {
			_ = zw.Close()
			return apperror.Internal(err)
		}
	}
	return zw.Close()
}
