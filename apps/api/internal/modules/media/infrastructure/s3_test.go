package infrastructure

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// fakeDeleter records DeleteObject calls without touching any bucket.
type fakeDeleter struct {
	deleted []string
}

func (f *fakeDeleter) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.deleted = append(f.deleted, *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}

func newGuardedDriver() (*S3Driver, *fakeDeleter) {
	fake := &fakeDeleter{}
	d := &S3Driver{
		bucket: "mini-erp", deleter: fake,
		uploadPrefixes:    []string{"vioni/production-v2/upload"},
		protectedPrefixes: []string{"vioni/production/upload"},
	}
	return d, fake
}

// TestS3RemoveGuards (F1-9): Archive must never delete objects outside the
// deployment's own upload area or inside a protected legacy prefix.
func TestS3RemoveGuards(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name    string
		key     string
		deleted bool
	}{
		{"key deployment sendiri dihapus", "vioni/production-v2/upload/product/a.jpg", true},
		{"key legacy dilindungi", "vioni/production/upload/2024/01/a.jpg", false},
		{"key asing di luar prefix ditolak", "other-bucket/x.jpg", false},
		{"prefix mirip tanpa batas slash ditolak", "vioni/production-v2/upload-evil/a.jpg", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, fake := newGuardedDriver()
			d.Remove(ctx, c.key)
			if got := len(fake.deleted) > 0; got != c.deleted {
				t.Fatalf("Remove(%q): dihapus=%v, want %v", c.key, got, c.deleted)
			}
		})
	}
	t.Run("protected mirip tanpa batas slash tidak melindungi", func(t *testing.T) {
		fake := &fakeDeleter{}
		d := &S3Driver{bucket: "mini-erp", deleter: fake,
			uploadPrefixes:    []string{"vioni"},
			protectedPrefixes: []string{"vioni/prod"}}
		d.Remove(ctx, "vioni/prod-evil/a.jpg")
		if len(fake.deleted) != 1 {
			t.Fatal("tanpa batas slash bukan containment: key harus dihapus")
		}
	})
}

// TestS3RemoveWithoutUploadPrefixes keeps the historical allow-all behavior
// when the deployment did not configure its own prefixes (dev/local parity),
// while protected prefixes are still honoured.
func TestS3RemoveWithoutUploadPrefixes(t *testing.T) {
	ctx := context.Background()
	fake := &fakeDeleter{}
	d := &S3Driver{bucket: "mini-erp", deleter: fake, protectedPrefixes: []string{"vioni/production/upload"}}
	d.Remove(ctx, "bebas/a.jpg")
	if len(fake.deleted) != 1 {
		t.Fatal("tanpa UploadPrefixes, key biasa harus tetap dihapus (paritas lama)")
	}
	d.Remove(ctx, "vioni/production/upload/a.jpg")
	if len(fake.deleted) != 1 {
		t.Fatal("prefix protected harus tetap dilindungi walau UploadPrefixes kosong")
	}
}

// TestUnderPrefixBoundary pins the slash-boundary rule: a shared string
// prefix without the "/" separator is not containment.
func TestUnderPrefixBoundary(t *testing.T) {
	if underPrefix("vioni/prod-evil/a.jpg", "vioni/prod") {
		t.Fatal("tanpa batas slash bukan containment")
	}
	if !underPrefix("vioni/prod/a.jpg", "vioni/prod") {
		t.Fatal("anak prefix harus containment")
	}
	if !underPrefix("vioni/prod", "vioni/prod") {
		t.Fatal("prefix itu sendiri harus containment")
	}
}
