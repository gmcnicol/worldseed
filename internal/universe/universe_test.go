package universe

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateUsesOpaqueUniverseID(t *testing.T) {
	created, err := Create(context.Background(), CreateOptions{
		DataDir: t.TempDir(),
		Name:    "Human Chosen Name",
		Seed:    7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(created.Path, "/human-chosen-name/") {
		t.Fatalf("path should use opaque universe id, got %s", created.Path)
	}
	if !strings.Contains(created.Path, "/"+created.ID+"/universe.sqlite") {
		t.Fatalf("path should include universe id %s, got %s", created.ID, created.Path)
	}
	resolved, err := ResolveDatabasePath(context.Background(), filepath.Dir(filepath.Dir(filepath.Dir(created.Path))), created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != created.Path {
		t.Fatalf("expected id to resolve to %s, got %s", created.Path, resolved)
	}
}

func TestNewIDReturnsUUIDLikeIdentifier(t *testing.T) {
	id, err := NewID(strings.NewReader("0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	if id != "30313233-3435-4637-b839-616263646566" {
		t.Fatalf("unexpected id: %s", id)
	}
}
