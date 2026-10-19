package application

import (
	"strings"

	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/shared/apperror"
)

// variantPlanInput is everything planVariants decides from. It is a pure
// function so the stock-safety rules are unit-testable without a database.
type variantPlanInput struct {
	ProductCode    string // new (upper-cased) product code
	OldProductCode string // "" on create
	ProductName    string
	Existing       []*contracts.Variant // any status, as stored
	// Payload nil = keep the stored variant set; non-nil empty = "no
	// explicit variants" (revert to the hidden default).
	Payload []VariantInput
	States  map[int64]contracts.VariantStockState
}

const msgVariantHasStock = "Varian masih punya stok. Kosongkan stok varian sebelum dihapus atau disembunyikan."

func variantErr(msg string) *apperror.AppError {
	return apperror.Validation("", []apperror.FieldError{{Field: "variants", Message: msg}})
}

func normCode(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// planVariants returns the rows to write (ID 0 = insert, else update in
// place). It never lets a variant that holds stock or has history disappear
// or be renamed behind the caller's back (F3b-3.1).
func planVariants(in variantPlanInput) ([]*contracts.Variant, error) {
	var hiddenDefault *contracts.Variant
	for _, v := range in.Existing {
		if v.IsHidden && v.IsDefault && hiddenDefault == nil {
			hiddenDefault = v
		}
	}
	state := func(v *contracts.Variant) contracts.VariantStockState { return in.States[v.ID] }
	clone := func(v *contracts.Variant) *contracts.Variant { c := *v; return &c }

	// followProductCode keeps the hidden default scannable by the product
	// code, unless stock history froze its code (it is a stable key then).
	followProductCode := func(v *contracts.Variant) {
		v.Name = in.ProductName
		if in.OldProductCode != "" && v.Code == in.OldProductCode && in.ProductCode != in.OldProductCode &&
			!state(v).HasHistory {
			v.Code = in.ProductCode
		}
	}

	if in.Payload == nil {
		if len(in.Existing) == 0 {
			return []*contracts.Variant{{Code: in.ProductCode, Name: in.ProductName,
				IsDefault: true, IsHidden: true, Status: "active"}}, nil
		}
		if hiddenDefault == nil {
			return nil, nil
		}
		c := clone(hiddenDefault)
		followProductCode(c)
		if c.Code == hiddenDefault.Code && c.Name == hiddenDefault.Name {
			return nil, nil
		}
		return []*contracts.Variant{c}, nil
	}

	var out []*contracts.Variant
	// Empty explicit set: back to the hidden default.
	if len(in.Payload) == 0 {
		for _, v := range in.Existing {
			if v.IsHidden || v.Status == "archived" {
				continue
			}
			st := state(v)
			if st.OnHand > 0 {
				return nil, variantErr(msgVariantHasStock)
			}
			if st.HasHistory {
				return nil, variantErr("Varian '" + v.Code + "' sudah punya riwayat stok dan tidak dapat dihilangkan")
			}
			c := clone(v)
			c.Status = "archived"
			out = append(out, c)
		}
		if hiddenDefault == nil {
			out = append(out, &contracts.Variant{Code: in.ProductCode, Name: in.ProductName,
				IsDefault: true, IsHidden: true, Status: "active"})
			return out, nil
		}
		c := clone(hiddenDefault)
		c.Status, c.IsDefault = "active", true
		followProductCode(c)
		out = append(out, c)
		return out, nil
	}

	byID := map[int64]*contracts.Variant{}
	byCode := map[string]*contracts.Variant{}
	for _, v := range in.Existing {
		byID[v.ID] = v
		byCode[normCode(v.Code)] = v
	}
	matched := map[int64]bool{}
	activeRows := 0
	flaggedDefault := false
	for _, row := range in.Payload {
		code := normCode(row.Code)
		var cur *contracts.Variant
		if row.ID > 0 {
			cur = byID[row.ID]
			if cur == nil {
				return nil, variantErr("varian id " + itoa(row.ID) + " tidak ditemukan pada produk ini")
			}
		} else {
			cur = byCode[code]
		}
		status := row.Status
		if status == "" {
			status = "active"
		}
		var c *contracts.Variant
		if cur != nil {
			if matched[cur.ID] {
				return nil, variantErr("varian '" + code + "' ganda")
			}
			matched[cur.ID] = true
			st := state(cur)
			if code != cur.Code && st.HasHistory {
				return nil, variantErr("Kode varian '" + cur.Code + "' terkunci karena sudah ada riwayat stok (buat varian baru, jangan ubah kodenya)")
			}
			if status != "active" && st.OnHand > 0 {
				return nil, variantErr(msgVariantHasStock)
			}
			c = clone(cur)
		} else {
			c = &contracts.Variant{}
		}
		c.Code, c.Name, c.Barcode = code, strings.TrimSpace(row.Name), strings.TrimSpace(row.Barcode)
		// A form that loads every variant back (the implicit default
		// included) must not turn the hidden default into a visible one.
		c.IsHidden = row.Hidden != nil && *row.Hidden
		if cur != nil && cur.IsHidden && row.Hidden == nil {
			c.IsHidden = true
		}
		c.Status, c.SortOrder = status, row.SortOrder
		c.Attributes = row.Attributes
		if c.IsHidden && cur != nil && cur.IsDefault {
			c.IsDefault = true
		} else {
			c.IsDefault = row.IsDefault && status == "active" && !flaggedDefault
			if c.IsDefault {
				flaggedDefault = true
			}
		}
		if status == "active" {
			activeRows++
		}
		out = append(out, c)
	}
	if activeRows == 0 {
		return nil, variantErr("minimal satu varian harus aktif")
	}
	// Stored variants the payload left out.
	retainedDefault := false
	for _, v := range in.Existing {
		if matched[v.ID] {
			continue
		}
		st := state(v)
		if v.IsHidden && v.IsDefault {
			// The implicit default steps aside for explicit variants. Stock on
			// it would be orphaned from every visible variant: refuse.
			if v.Status == "active" {
				if st.OnHand > 0 {
					return nil, variantErr("Produk masih punya stok pada varian bawaan. Kosongkan atau pindahkan stoknya sebelum memakai varian")
				}
				c := clone(v)
				if st.HasHistory {
					retainedDefault = true // stays scannable through the product code
				} else {
					c.Status = "archived"
				}
				out = append(out, c)
			}
			continue
		}
		if v.Status == "archived" {
			continue
		}
		if st.OnHand > 0 {
			return nil, variantErr(msgVariantHasStock)
		}
		if st.HasHistory {
			return nil, variantErr("Varian '" + v.Code + "' punya riwayat stok: kirim ulang dengan status 'inactive' untuk menyembunyikannya, tidak dihapus diam-diam")
		}
		c := clone(v)
		c.Status = "archived"
		out = append(out, c)
	}
	// Exactly one default: the flagged row; else the retained hidden default;
	// else the first active row.
	hasHiddenDefault := retainedDefault
	for _, c := range out {
		if c.IsHidden && c.IsDefault && c.Status == "active" {
			hasHiddenDefault = true
		}
	}
	if flaggedDefault {
		for _, c := range out {
			if c.IsHidden {
				c.IsDefault = false
			}
		}
	} else if !hasHiddenDefault {
		for _, c := range out {
			if !c.IsHidden && c.Status == "active" {
				c.IsDefault = true
				break
			}
		}
	}
	// A hidden default with a retained identity keeps following the product.
	for _, c := range out {
		if c.IsHidden && c.IsDefault && c.Status == "active" {
			followProductCode(c)
		}
	}
	return out, nil
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
