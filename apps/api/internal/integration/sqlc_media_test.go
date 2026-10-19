package integration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	mediacontracts "mini-erp/internal/modules/media/contracts"
	mediainfra "mini-erp/internal/modules/media/infrastructure"
)

// Characterization tests for the media Repository (F8-18): they pin the
// behaviour of every public method so the sqlc rewrite is provably identical.
func TestSqlcMediaRepository(t *testing.T) {
	ctx := context.Background()
	freshDB(t)
	repo := mediainfra.NewRepository(testMods.db)

	mk := func(owner string, ownerID int64, key string, w, h int, orig int64, origin string) *mediacontracts.MediaFile {
		return &mediacontracts.MediaFile{OwnerType: owner, OwnerID: ownerID, Key: key, OriginalName: key + ".jpg",
			MIME: "image/jpeg", SizeBytes: 100, Width: w, Height: h, OriginalSizeBytes: orig, Origin: origin}
	}

	// First file of an owner is primary; later ones are not; origin defaults to app.
	a := mk("product", 1, "k/a", 640, 480, 2000, "")
	idA, err := repo.Insert(ctx, a, 9)
	if err != nil || idA == 0 || !a.IsPrimary {
		t.Fatalf("insert a: id=%d primary=%v err=%v", idA, a.IsPrimary, err)
	}
	b := mk("product", 1, "k/b", 0, 0, 0, mediacontracts.OriginLegacy)
	idB, err := repo.Insert(ctx, b, 0)
	if err != nil || b.IsPrimary {
		t.Fatalf("insert b: primary=%v err=%v", b.IsPrimary, err)
	}
	c := mk("product", 1, "k/c", 10, 10, 5, "")
	idC, err := repo.Insert(ctx, c, 0)
	if err != nil || c.IsPrimary {
		t.Fatalf("insert c: %v", err)
	}
	// Other owner / other type keep their own primary.
	d := mk("product", 2, "k/d", 0, 0, 0, "")
	if _, err := repo.Insert(ctx, d, 0); err != nil || !d.IsPrimary {
		t.Fatalf("insert d: %v", err)
	}
	e := mk("payment_proof", 1, "k/e", 0, 0, 0, "")
	if _, err := repo.Insert(ctx, e, 0); err != nil || !e.IsPrimary {
		t.Fatalf("insert e: %v", err)
	}
	// Duplicate key violates the unique index.
	if _, err := repo.Insert(ctx, mk("product", 3, "k/a", 0, 0, 0, ""), 0); err == nil {
		t.Fatalf("duplicate key must fail")
	}

	// GetByID: found, wrong owner, wrong type, missing.
	got, err := repo.GetByID(ctx, "product", 1, idA)
	if err != nil || got == nil {
		t.Fatalf("get a: %v %v", got, err)
	}
	if got.Key != "k/a" || got.OriginalName != "k/a.jpg" || got.MIME != "image/jpeg" || got.SizeBytes != 100 ||
		!got.IsPrimary || got.Origin != "app" || got.Width != 640 || got.Height != 480 || got.OriginalSizeBytes != 2000 || got.ID != idA || got.OwnerID != 1 || got.OwnerType != "product" {
		t.Fatalf("get a mismatch: %+v", got)
	}
	gb, _ := repo.GetByID(ctx, "product", 1, idB)
	if gb == nil || gb.Origin != "legacy" || gb.Width != 0 || gb.Height != 0 || gb.OriginalSizeBytes != 0 || gb.IsPrimary {
		t.Fatalf("get b (NULL dims) mismatch: %+v", gb)
	}
	for _, c := range []struct {
		typ      string
		own, fid int64
	}{{"product", 2, idA}, {"payment_proof", 1, idA}, {"product", 1, 999999}} {
		if f, err := repo.GetByID(ctx, c.typ, c.own, c.fid); err != nil || f != nil {
			t.Fatalf("get %+v want nil: %v %v", c, f, err)
		}
	}

	// List: primary first, then sort_order, id; unknown owner -> empty.
	list, err := repo.List(ctx, "product", 1)
	if err != nil || len(list) != 3 || list[0].ID != idA || list[1].ID != idB || list[2].ID != idC {
		t.Fatalf("list: %v %v", list, err)
	}
	if l, err := repo.List(ctx, "product", 404); err != nil || len(l) != 0 {
		t.Fatalf("list empty: %v %v", l, err)
	}

	// PrimaryKeysByOwners: empty, partial, unknown, large (>1000 ids).
	if m, err := repo.PrimaryKeysByOwners(ctx, "product", nil); err != nil || len(m) != 0 {
		t.Fatalf("pk empty: %v %v", m, err)
	}
	m, err := repo.PrimaryKeysByOwners(ctx, "product", []int64{1, 2, 3})
	if err != nil || len(m) != 2 || m[1] != "k/a" || m[2] != "k/d" {
		t.Fatalf("pk: %v %v", m, err)
	}
	if m, _ := repo.PrimaryKeysByOwners(ctx, "payment_proof", []int64{1, 2}); len(m) != 1 || m[1] != "k/e" {
		t.Fatalf("pk type scope: %v", m)
	}
	big := make([]int64, 0, 2500)
	for i := int64(1); i <= 2500; i++ {
		big = append(big, i)
	}
	if m, err := repo.PrimaryKeysByOwners(ctx, "product", big); err != nil || len(m) != 2 {
		t.Fatalf("pk big: %v %v", m, err)
	}

	// SetPrimary: switches atomically; missing file -> sql.ErrNoRows and no change.
	if err := repo.SetPrimary(ctx, "product", 1, idC); err != nil {
		t.Fatalf("setprimary: %v", err)
	}
	list, _ = repo.List(ctx, "product", 1)
	if list[0].ID != idC || !list[0].IsPrimary || list[1].IsPrimary || list[2].IsPrimary {
		t.Fatalf("after setprimary: %+v", list)
	}
	if err := repo.SetPrimary(ctx, "product", 1, 999999); err != sql.ErrNoRows {
		t.Fatalf("setprimary missing want ErrNoRows got %v", err)
	}
	list, _ = repo.List(ctx, "product", 1)
	if list[0].ID != idC || !list[0].IsPrimary {
		t.Fatalf("failed setprimary must roll back: %+v", list)
	}

	// Delete: missing -> empty; non-primary delete keeps primary; primary delete
	// promotes the oldest (sort_order, id) remaining.
	if k, o, err := repo.Delete(ctx, "product", 1, 999999); err != nil || k != "" || o != "" {
		t.Fatalf("delete missing: %q %q %v", k, o, err)
	}
	if k, o, err := repo.Delete(ctx, "product", 2, idA); err != nil || k != "" || o != "" {
		t.Fatalf("delete foreign owner: %q %q %v", k, o, err)
	}
	if k, o, err := repo.Delete(ctx, "product", 1, idA); err != nil || k != "k/a" || o != "app" {
		t.Fatalf("delete non-primary: %q %q %v", k, o, err)
	}
	if k, o, err := repo.Delete(ctx, "product", 1, idC); err != nil || k != "k/c" || o != "app" {
		t.Fatalf("delete primary: %q %q %v", k, o, err)
	}
	list, _ = repo.List(ctx, "product", 1)
	if len(list) != 1 || list[0].ID != idB || !list[0].IsPrimary || list[0].Origin != "legacy" {
		t.Fatalf("promotion after primary delete: %+v", list)
	}
	// Deleting the last file leaves nothing and must not promote other owners.
	if _, _, err := repo.Delete(ctx, "product", 1, idB); err != nil {
		t.Fatalf("delete last: %v", err)
	}
	if g, _ := repo.GetByID(ctx, "product", 2, mustFirstID(t, repo, "product", 2)); g == nil || !g.IsPrimary {
		t.Fatalf("other owner untouched: %+v", g)
	}

	// Concurrent first uploads on one owner yield exactly one primary.
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		go func(i int) {
			_, err := repo.Insert(ctx, mk("product", 77, fmt.Sprintf("k/race%d", i), 0, 0, 0, ""), 0)
			errs <- err
		}(i)
	}
	for i := 0; i < 6; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("race insert: %v", err)
		}
	}
	rl, _ := repo.List(ctx, "product", 77)
	prim := 0
	for _, f := range rl {
		if f.IsPrimary {
			prim++
		}
	}
	if len(rl) != 6 || prim != 1 {
		t.Fatalf("race: %d files, %d primary", len(rl), prim)
	}
}

func mustFirstID(t *testing.T, repo *mediainfra.Repository, owner string, ownerID int64) int64 {
	t.Helper()
	l, err := repo.List(context.Background(), owner, ownerID)
	if err != nil || len(l) == 0 {
		t.Fatalf("list %s/%d: %v %v", owner, ownerID, l, err)
	}
	return l[0].ID
}
