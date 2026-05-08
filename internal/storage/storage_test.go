package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/gmcnicol/worldseed/internal/storage"
	"github.com/gmcnicol/worldseed/internal/universe"
)

func TestRecordClientKeyAssociatesKeyWithUniverse(t *testing.T) {
	ctx := context.Background()
	created, err := universe.Create(ctx, universe.CreateOptions{
		DataDir: t.TempDir(),
		Name:    "archive",
		Seed:    13,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(created.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	u, err := store.LoadUniverse(ctx)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	key := storage.ClientKey{
		UniverseID:  u.ID,
		Username:    "operator",
		Fingerprint: "SHA256:example",
		PublicKey:   "ssh-ed25519 AAAAexample",
		FirstSeenAt: now,
		LastSeenAt:  now,
		RemoteAddr:  "127.0.0.1:48000",
	}
	if err := store.RecordClientKey(ctx, key); err != nil {
		t.Fatal(err)
	}

	keys, err := store.ClientKeys(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected one client key, got %d", len(keys))
	}
	if keys[0].UniverseID != u.ID || keys[0].Fingerprint != key.Fingerprint {
		t.Fatalf("key was not associated with universe: %#v", keys[0])
	}

	key.LastSeenAt = now.Add(time.Hour)
	key.RemoteAddr = "127.0.0.1:48001"
	if err := store.RecordClientKey(ctx, key); err != nil {
		t.Fatal(err)
	}
	keys, err = store.ClientKeys(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected upserted client key, got %d rows", len(keys))
	}
	if keys[0].RemoteAddr != key.RemoteAddr {
		t.Fatalf("expected remote addr update, got %s", keys[0].RemoteAddr)
	}
}
