package application

import (
	"context"
	"fmt"
	"io"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/product/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
)

// importChunk is how many products one transaction carries.
const importChunk = 200

// CommitImport applies a workbook all-or-nothing at the validation gate: any
// error finding rejects the whole file with nothing written. Past the gate the
// writes are chunked and idempotent (upsert by code: running the same file
// again changes nothing), categories in one transaction, products 200 per
// transaction with a per-row fallback so one unlucky row (a race lost on a
// unique key) is reported instead of dropping its 199 neighbours.
func (s *Service) CommitImport(ctx context.Context, actorID int64, r io.Reader) (*ImportResult, error) {
	raw, err := io.ReadAll(io.LimitReader(r, maxImportBytes+1))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "gagal membaca file"}})
	}
	if len(raw) > maxImportBytes {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "file", Message: "maksimal 10 MB"}})
	}
	plan, err := s.buildImportPlan(ctx, raw)
	if err != nil {
		return nil, wrapErr(err)
	}
	if plan.Summary.Errors > 0 {
		var fields []apperror.FieldError
		for _, is := range plan.Issues {
			if is.Severity != "error" || len(fields) >= 20 {
				continue
			}
			fields = append(fields, apperror.FieldError{Field: "file",
				Message: fmt.Sprintf("%s baris %d (%s): %s", is.Sheet, is.Row, is.Column, is.Message)})
		}
		return nil, apperror.Validation(fmt.Sprintf(
			"Impor ditolak: %d galat pada file, tidak ada data yang ditulis. Perbaiki lalu pratinjau ulang.",
			plan.Summary.Errors), fields)
	}

	res := &ImportResult{Failures: []RowError{}, Notes: []string{}}
	if len(plan.Restored) > 0 {
		res.Notes = append(res.Notes, "kategori dipulihkan dari arsip: "+strings.Join(plan.Restored, ", "))
	}
	if len(plan.Categories) > 0 {
		writes := make([]infrastructure.CategoryWrite, 0, len(plan.Categories))
		for _, c := range plan.Categories {
			writes = append(writes, infrastructure.CategoryWrite{ID: c.ExistingID, Code: c.Code, Name: c.Name,
				ParentCode: c.Parent, SortOrder: c.Sort, Status: c.Status})
		}
		if err := s.repo.ApplyCategories(ctx, writes, actorID); err != nil {
			return nil, dberr.Map(err)
		}
		for _, c := range plan.Categories {
			if c.Action == "create" {
				res.Imported.CategoriesCreated++
			} else {
				res.Imported.CategoriesUpdated++
			}
		}
	}
	// Category codes -> ids now that every category exists.
	catID := map[string]int64{}
	if cats, err := s.repo.ListCategories(ctx); err == nil {
		for _, c := range cats {
			catID[strings.ToUpper(c.Code)] = c.ID
		}
	} else {
		return nil, apperror.Internal(err)
	}
	var writes []infrastructure.ProductWrite
	byRow := map[int]*importProduct{}
	for _, p := range plan.Products {
		if p.CatSet {
			p.Write.CategoryID = nil
			if p.CatCode != "" {
				id := catID[p.CatCode]
				p.Write.CategoryID = &id
			}
		}
		byRow[p.Row] = p
		writes = append(writes, infrastructure.ProductWrite{Product: p.Write, Row: p.Row})
	}
	count := func(p *importProduct) {
		if p.Action == "create" {
			res.Imported.ProductsCreated++
			res.Inserted++
		} else {
			res.Imported.ProductsUpdated++
		}
	}
	for start := 0; start < len(writes); start += importChunk {
		chunk := writes[start:min(start+importChunk, len(writes))]
		if err := s.repo.WriteProducts(ctx, chunk, actorID); err == nil {
			for _, w := range chunk {
				count(byRow[w.Row])
			}
			continue
		}
		// The chunk rolled back as a whole; retry row by row to isolate the culprit.
		for _, w := range chunk {
			if err := s.repo.WriteProducts(ctx, []infrastructure.ProductWrite{w}, actorID); err != nil {
				reason := "gagal menyimpan"
				if ae := dberr.Map(err); ae != nil && ae.Message != "" {
					reason = ae.Message
				}
				res.Failures = append(res.Failures, RowError{Sheet: sheetTitle[sheetProducts], Row: w.Row,
					Column: "Kode Produk", Value: byRow[w.Row].Code, Reason: reason})
				continue
			}
			count(byRow[w.Row])
		}
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "products.bulk_import", Entity: "product", ActorID: actorID,
		Note: fmt.Sprintf("kategori +%d/~%d, produk +%d/~%d, gagal %d",
			res.Imported.CategoriesCreated, res.Imported.CategoriesUpdated,
			res.Imported.ProductsCreated, res.Imported.ProductsUpdated, len(res.Failures)),
		Metadata: map[string]any{"summary": plan.Summary, "imported": res.Imported, "failures": len(res.Failures)}})
	return res, nil
}
