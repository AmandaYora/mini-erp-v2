package application

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/modules/stock/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// OpeningSheet is the sheet name the template is written with.
const OpeningSheet = "Stok Awal"

// openingSheetAliases are the sheet names the import accepts (normalized):
// the old English "opening" and the legacy Indonesian names. A workbook with a
// single sheet is read whatever it is called.
var openingSheetAliases = []string{"opening", "stok awal", "stock opening", "opening stock", "migrasi stok"}

// OpeningHeaders defines the template column order (Indonesian, as the
// legacy workbook wrote them). The last column is informational only.
var OpeningHeaders = []string{"Kode Barang", "Kode Varian", "Kode Lokasi", "Jumlah Stok Awal", "Nama Barang"}

// openingHeaderAliases maps each canonical column to the header spellings the
// import accepts, normalized (lower case, single spaces, no "?"): the revamp's
// English names and the legacy Indonesian ones.
var openingHeaderAliases = map[string][]string{
	"product_code":  {"product_code", "kode barang", "kode produk", "kode item", "sku"},
	"variant_code":  {"variant_code", "kode varian", "varian"},
	"location_code": {"location_code", "kode lokasi", "kode gudang"},
	"qty":           {"qty", "jumlah stok awal", "stok awal", "qty awal", "quantity_on_hand", "qty_on_hand", "jumlah"},
}

func normalizeHeader(s string) string {
	s = strings.ToLower(strings.ReplaceAll(s, "?", ""))
	return strings.Join(strings.Fields(s), " ")
}

// parseFlexibleQty reads "1.000,5", "1,000.5", "12,5" and "12" alike (legacy
// parseFlexibleNumber). ok=false for anything that is not a number.
func parseFlexibleQty(raw string) (float64, bool) {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if (r >= '0' && r <= '9') || r == ',' || r == '.' || r == '-' {
			b.WriteRune(r)
		}
	}
	c := b.String()
	if c == "" {
		return 0, false
	}
	hasComma, hasDot := strings.Contains(c, ","), strings.Contains(c, ".")
	switch {
	case hasComma && hasDot:
		if strings.LastIndex(c, ",") > strings.LastIndex(c, ".") {
			c = strings.ReplaceAll(strings.ReplaceAll(c, ".", ""), ",", ".")
		} else {
			c = strings.ReplaceAll(c, ",", "")
		}
	case hasComma:
		c = strings.Replace(c, ",", ".", 1)
	}
	f, err := strconv.ParseFloat(c, 64)
	return f, err == nil
}

// OpeningRowError pinpoints one rejected row.
type OpeningRowError struct {
	Row    int    `json:"row"`
	Column string `json:"column"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// OpeningPreview is the dry-run result: nothing is written. Skipped rows are
// valid but write nothing (zero quantity, or the position already has history).
type OpeningPreview struct {
	ValidRows int               `json:"validRows"`
	Skipped   int               `json:"skipped"`
	Errors    []OpeningRowError `json:"errors"`
	// Rows is the per-item detail of the valid and skipped rows (capped at
	// openingPreviewRows; RowsTruncated says more exist); TotalQty/TotalValue sum
	// the rows that WOULD post (value = qty x purchase cost, whole rupiah).
	Rows          []OpeningPreviewRow `json:"rows"`
	RowsTruncated bool                `json:"rowsTruncated"`
	TotalQty      float64             `json:"totalQty"`
	TotalValue    int64               `json:"totalValue"`
	Warnings      []string            `json:"warnings"`
}

// OpeningPreviewRow is one validated row of the preview.
type OpeningPreviewRow struct {
	Row          int     `json:"row"`
	ProductCode  string  `json:"productCode"`
	ProductName  string  `json:"productName"`
	VariantCode  string  `json:"variantCode"`
	LocationCode string  `json:"locationCode"`
	Qty          float64 `json:"qty"`
	UnitCost     float64 `json:"unitCost"`
	Value        int64   `json:"value"`
	// Status is "post" (will be booked) or "skip" (zero qty / position has history).
	Status string `json:"status"`
	Note   string `json:"note"`
}

// openingPreviewRows caps the detail rows returned by one preview.
const openingPreviewRows = 500

// OpeningResult is the commit outcome.
type OpeningResult struct {
	Posted   int               `json:"posted"`
	Skipped  int               `json:"skipped"`
	Failures []OpeningRowError `json:"failures"`
}

type openingRow struct {
	line         int
	productCode  string
	variantCode  string
	locationCode string
	qtyRaw       string
}

// openingTemplateMax caps the products prefilled into the template.
const openingTemplateMax = 5000

// suggestedOpeningLocation picks the location a fresh product is most likely to
// be stored in: the same priority the allocation uses (picking area, primary,
// the branch default, then sort order) among active non-damaged leaf locations.
func (s *Service) suggestedOpeningLocation(ctx context.Context, branchID int64) (string, error) {
	locs, err := s.repo.ListLocations(ctx, branchID, "active")
	if err != nil {
		return "", apperror.Internal(err)
	}
	var best *contracts.Location
	rank := func(l *contracts.Location) [4]int {
		b := func(v bool) int {
			if v {
				return 0
			}
			return 1
		}
		return [4]int{b(l.IsPickingArea), b(l.IsPrimary), b(l.IsSystem && l.Kind == "normal"), l.SortOrder}
	}
	for _, l := range locs {
		if l.Kind == contracts.KindDamaged {
			continue
		}
		if leaf, err := s.repo.IsLeaf(ctx, l.ID); err != nil || !leaf {
			continue
		}
		if best == nil {
			best = l
			continue
		}
		rb, rl := rank(best), rank(l)
		for k := 0; k < 4; k++ {
			if rl[k] != rb[k] {
				if rl[k] < rb[k] {
					best = l
				}
				break
			}
		}
	}
	if best == nil {
		return "", nil
	}
	return best.Code, nil
}

// OpeningTemplate writes the opening workbook to w, prefilled with the tracked
// products of the catalog and a suggested location each (quantity left blank:
// blank rows are skipped), so the user only types the counts.
func (s *Service) OpeningTemplate(ctx context.Context, branchID int64, w io.Writer) error {
	f := excelize.NewFile()
	if err := f.SetSheetName("Sheet1", OpeningSheet); err != nil {
		return apperror.Internal(err)
	}
	for i, h := range OpeningHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(OpeningSheet, cell, h)
	}
	loc, err := s.suggestedOpeningLocation(ctx, branchID)
	if err != nil {
		return err
	}
	row := 2
	var after int64
	for row-2 < openingTemplateMax {
		page, err := s.products.ListStockedAfter(ctx, after, 500)
		if err != nil {
			return apperror.Internal(err)
		}
		if len(page) == 0 {
			break
		}
		for _, p := range page {
			if row-2 >= openingTemplateMax {
				break
			}
			for c, v := range []string{p.Code, "", loc, "", p.Name} {
				if v == "" {
					continue
				}
				cell, _ := excelize.CoordinatesToCellName(c+1, row)
				_ = f.SetCellValue(OpeningSheet, cell, v)
			}
			row++
		}
		after = page[len(page)-1].ID
		if len(page) < 500 {
			break
		}
	}
	if row == 2 { // empty catalog: keep one example row
		for c, v := range []string{"BRG-001", "", "GDG", "100", "Contoh barang"} {
			cell, _ := excelize.CoordinatesToCellName(c+1, 2)
			_ = f.SetCellValue(OpeningSheet, cell, v)
		}
	}
	_ = f.SetColWidth(OpeningSheet, "A", "A", 18)
	_ = f.SetColWidth(OpeningSheet, "C", "D", 18)
	_ = f.SetColWidth(OpeningSheet, "E", "E", 36)
	if err := f.Write(w); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

func openOpeningBook(r io.Reader) (*excelize.File, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 10<<20))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "gagal membaca file"}})
	}
	if len(raw) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "file kosong"}})
	}
	f, err := excelize.OpenReader(bytes.NewReader(raw))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "bukan file Excel yang valid"}})
	}
	return f, nil
}

func readOpeningRows(f *excelize.File) ([]openingRow, error) {
	sheet := ""
	names := f.GetSheetList()
	for _, n := range names {
		for _, a := range openingSheetAliases {
			if normalizeHeader(n) == a {
				sheet = n
				break
			}
		}
		if sheet != "" {
			break
		}
	}
	if sheet == "" && len(names) == 1 {
		sheet = names[0]
	}
	if sheet == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file",
			Message: `sheet "Stok Awal" tidak ditemukan (nama sheet yang dikenal: Stok Awal, Opening, Stock Opening, Migrasi Stok)`}})
	}
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "sheet tidak dapat dibaca"}})
	}
	if len(rows) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "sheet kosong"}})
	}
	// canonical column -> index, through the header aliases.
	idx := map[string]int{}
	for i, h := range rows[0] {
		n := normalizeHeader(h)
		for canon, aliases := range openingHeaderAliases {
			for _, a := range aliases {
				if n == a {
					if _, taken := idx[canon]; !taken {
						idx[canon] = i
					}
				}
			}
		}
	}
	for _, canon := range []string{"product_code", "location_code", "qty"} {
		if _, ok := idx[canon]; !ok {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "file",
				Message: "kolom " + openingColumnLabel(canon) + " tidak ditemukan di baris judul"}})
		}
	}
	var out []openingRow
	for n, row := range rows[1:] {
		blank := true
		for _, c := range row {
			if strings.TrimSpace(c) != "" {
				blank = false
				break
			}
		}
		if blank {
			continue
		}
		get := func(h string) string {
			if i, ok := idx[h]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		out = append(out, openingRow{
			line: n + 2, productCode: strings.ToUpper(get("product_code")),
			variantCode:  strings.ToUpper(get("variant_code")),
			locationCode: strings.ToUpper(get("location_code")), qtyRaw: get("qty"),
		})
	}
	return out, nil
}

func openingColumnLabel(canon string) string {
	switch canon {
	case "product_code":
		return `"Kode Barang"`
	case "location_code":
		return `"Kode Lokasi"`
	case "qty":
		return `"Jumlah Stok Awal"`
	}
	return canon
}

type resolvedOpening struct {
	productName string
	line        int
	branchID    int64
	productID   int64
	variantID   int64
	locationID  int64
	qty         float64
	unitCost    float64 // catalog purchase price per base unit (the cost basis)
}

// resolveOpeningRow validates one row against live master data.
func (s *Service) resolveOpeningRow(ctx context.Context, branchID int64, row openingRow) (*resolvedOpening, *OpeningRowError) {
	fail := func(col, reason string) *OpeningRowError {
		val := row.productCode
		if col == "location_code" {
			val = row.locationCode
		} else if col == "qty" {
			val = row.qtyRaw
		} else if col == "variant_code" {
			val = row.variantCode
		}
		return &OpeningRowError{Row: row.line, Column: col, Value: val, Reason: reason}
	}
	if row.productCode == "" {
		return nil, fail("product_code", "wajib diisi")
	}
	product, err := s.products.GetByCode(ctx, row.productCode)
	if err != nil {
		return nil, &OpeningRowError{Row: row.line, Column: "product_code", Value: row.productCode, Reason: "gagal membaca produk"}
	}
	if product == nil {
		return nil, fail("product_code", "produk tidak ditemukan")
	}
	if product.Status != "active" {
		return nil, fail("product_code", "produk sudah diarsipkan")
	}
	if !product.Tracked {
		return nil, fail("product_code", "produk non-stok tidak punya saldo")
	}
	costs, cerr := s.products.PurchaseBaseCostsByIDs(ctx, []int64{product.ID})
	if cerr != nil {
		return nil, &OpeningRowError{Row: row.line, Column: "product_code", Value: row.productCode, Reason: "gagal membaca harga beli"}
	}
	unitCost := costs[product.ID]
	if unitCost <= 0 {
		return nil, fail("product_code", "harga beli belum diisi di master produk (dipakai sebagai harga modal)")
	}
	variantID := int64(0)
	if row.variantCode != "" {
		found := false
		for _, v := range product.Variants {
			if v.Code == row.variantCode {
				variantID, found = v.ID, true
				break
			}
		}
		if !found {
			return nil, fail("variant_code", "varian tidak dikenal")
		}
	} else {
		for _, v := range product.Variants {
			if v.IsDefault {
				variantID = v.ID
				break
			}
		}
		if variantID == 0 && len(product.Variants) > 0 {
			variantID = product.Variants[0].ID
		}
	}
	if row.locationCode == "" {
		return nil, fail("location_code", "wajib diisi")
	}
	loc, err := s.repo.LocationByCode(ctx, branchID, row.locationCode)
	if err != nil {
		return nil, &OpeningRowError{Row: row.line, Column: "location_code", Value: row.locationCode, Reason: "gagal membaca lokasi"}
	}
	if loc == nil || loc.Status != "active" {
		return nil, fail("location_code", "lokasi tidak ditemukan")
	}
	leaf, err := s.repo.IsLeaf(ctx, loc.ID)
	if err != nil {
		return nil, &OpeningRowError{Row: row.line, Column: "location_code", Value: row.locationCode, Reason: "gagal membaca lokasi"}
	}
	if !leaf {
		return nil, fail("location_code", "bukan lokasi daun")
	}
	// A blank quantity is "not counted yet" (the prefilled template leaves them
	// empty): it validates as zero and is skipped, never booked.
	qty := 0.0
	if strings.TrimSpace(row.qtyRaw) != "" {
		var ok bool
		if qty, ok = parseFlexibleQty(row.qtyRaw); !ok || qty < 0 {
			return nil, fail("qty", "harus angka ≥ 0")
		}
	}
	return &resolvedOpening{productName: product.Name, line: row.line, branchID: branchID, productID: product.ID,
		variantID: variantID, locationID: loc.ID, qty: qty, unitCost: unitCost}, nil
}

// openingGuard refuses a stock opening once finance posted its own cutover
// balance: from then on stock changes go through adjustments.
func (s *Service) openingGuard(ctx context.Context, branchID int64) error {
	if s.gate == nil {
		return nil
	}
	posted, err := s.gate.OpeningPosted(ctx, branchID)
	if err != nil {
		return err
	}
	if posted {
		return apperror.Conflict("Saldo awal finance sudah diposting; gunakan Penyesuaian Stok")
	}
	return nil
}

// PreviewOpening dry-runs the workbook: every row validated, nothing written.
func (s *Service) PreviewOpening(ctx context.Context, branchID int64, r io.Reader) (*OpeningPreview, error) {
	f, err := openOpeningBook(r)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := readOpeningRows(f)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	if err := s.openingGuard(ctx, branchID); err != nil {
		return nil, err
	}
	moved, err := s.repo.MovedPositions(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	seen := map[string]bool{}
	prev := &OpeningPreview{Errors: []OpeningRowError{}, Rows: []OpeningPreviewRow{}, Warnings: []string{}}
	zero, history := 0, 0
	addRow := func(ro *resolvedOpening, row openingRow, status, note string) {
		if len(prev.Rows) >= openingPreviewRows {
			prev.RowsTruncated = true
			return
		}
		prev.Rows = append(prev.Rows, OpeningPreviewRow{Row: row.line, ProductCode: row.productCode, ProductName: ro.productName,
			VariantCode: row.variantCode, LocationCode: row.locationCode, Qty: ro.qty, UnitCost: ro.unitCost,
			Value: int64(math.Round(ro.qty * ro.unitCost)), Status: status, Note: note})
	}
	for _, row := range rows {
		ro, roerr := s.resolveOpeningRow(ctx, branchID, row)
		if roerr != nil {
			prev.Errors = append(prev.Errors, *roerr)
			continue
		}
		key := infrastructure.PositionKey(ro.productID, ro.variantID, ro.locationID)
		if seen[key] {
			prev.Errors = append(prev.Errors, OpeningRowError{Row: row.line, Column: "product_code",
				Value: row.productCode, Reason: "posisi (produk, varian, lokasi) muncul dua kali dalam file"})
			continue
		}
		seen[key] = true
		switch {
		case moved[key]:
			prev.Skipped++
			history++
			addRow(ro, row, "skip", "posisi sudah punya mutasi")
			continue
		case ro.qty == 0:
			prev.Skipped++
			zero++
			addRow(ro, row, "skip", "jumlah kosong atau 0")
			continue
		}
		prev.ValidRows++
		prev.TotalQty += ro.qty
		prev.TotalValue += int64(math.Round(ro.qty * ro.unitCost))
		addRow(ro, row, "post", "")
	}
	if history > 0 {
		prev.Warnings = append(prev.Warnings, strconv.Itoa(history)+" baris dilewati karena posisinya sudah punya mutasi (tidak dibukukan ulang)")
	}
	if zero > 0 {
		prev.Warnings = append(prev.Warnings, strconv.Itoa(zero)+" baris dilewati karena jumlahnya kosong atau 0")
	}
	if prev.RowsTruncated {
		prev.Warnings = append(prev.Warnings, "rincian dibatasi "+strconv.Itoa(openingPreviewRows)+" baris pertama; total mencakup seluruh berkas")
	}
	return prev, nil
}

// openingChunk bounds one opening transaction (rows per commit).
const openingChunk = 500

// CommitOpening posts every valid row as an opening inflow. Rows whose
// position already has movements are skipped (resumable), never double-posted.
func (s *Service) CommitOpening(ctx context.Context, actorID, branchID int64, r io.Reader) (*OpeningResult, error) {
	f, err := openOpeningBook(r)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := readOpeningRows(f)
	if err != nil {
		if appErr, ok := err.(*apperror.AppError); ok {
			return nil, appErr
		}
		return nil, apperror.Internal(err)
	}
	if err := s.openingGuard(ctx, branchID); err != nil {
		return nil, err
	}
	if _, _, err := s.ensureDefaults(ctx, branchID); err != nil {
		return nil, err
	}
	// Positions that already have history — read once for the branch rather
	// than probed per row. Posting below adds to the set, so two rows in the
	// same file targeting one position still behave as before: the second
	// sees the first's movement and skips.
	moved, err := s.repo.MovedPositions(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	res := &OpeningResult{Failures: []OpeningRowError{}}
	// Rows post in chunks, each chunk in ONE transaction together with its own
	// audit row (ADR-0011): a failing trail rolls that chunk back, and a replay
	// finds the positions already moved so it writes neither stock nor trail.
	var pending []contracts.PostLine
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		n := len(pending)
		_, err := s.postWithHook(ctx, contracts.PostRequest{
			BranchID: branchID, Direction: "in", RefType: "opening", RefID: branchID,
			ActorID: actorID, Note: "Saldo awal", Lines: pending,
		}, func(ctx context.Context, tx *sql.Tx, _ contracts.PostResult) error {
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "stock_openings.commit", Entity: "stock_opening", BranchID: branchID, ActorID: actorID,
				Metadata: map[string]any{"positions": n},
			})
		})
		pending = nil
		return err
	}
	for _, row := range rows {
		ro, roerr := s.resolveOpeningRow(ctx, branchID, row)
		if roerr != nil {
			res.Failures = append(res.Failures, *roerr)
			continue
		}
		posKey := infrastructure.PositionKey(ro.productID, ro.variantID, ro.locationID)
		if moved[posKey] || ro.qty == 0 {
			// This exact position already has history (or nothing to post):
			// skip rather than double-post (resumable).
			res.Skipped++
			continue
		}
		// One idempotent document per branch: (opening, branch) + position is
		// the movement key, with the catalog purchase price as the exact cost.
		pending = append(pending, contracts.PostLine{ProductID: ro.productID, VariantID: ro.variantID,
			LocationID: ro.locationID, Qty: ro.qty, UnitCost: ro.unitCost})
		moved[posKey] = true
		res.Posted++
		if len(pending) >= openingChunk {
			if err := flush(); err != nil {
				return nil, err
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return res, nil
}
