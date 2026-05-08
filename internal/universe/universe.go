package universe

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gmcnicol/worldseed/internal/storage"
	"github.com/gmcnicol/worldseed/internal/timeline"
)

const (
	DefaultDataDir = "/var/lib/worldseed"
)

type CreateOptions struct {
	DataDir string
	Name    string
	Seed    int64
}

func DataDir(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("WORLDSEED_HOME"); env != "" {
		return env
	}
	return DefaultDataDir
}

func UniverseDir(dataDir, name string) string {
	return filepath.Join(DataDir(dataDir), "universes", Slug(name))
}

func DatabasePath(dataDir, name string) string {
	return filepath.Join(UniverseDir(dataDir, name), "universe.sqlite")
}

func HostKeyPath(dataDir, name string) string {
	return filepath.Join(UniverseDir(dataDir, name), "ssh_host_ed25519")
}

func SocketLabel(name string) string {
	return "worldseed:" + Slug(name)
}

func Slug(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case !lastDash:
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func SeedFromName(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("worldseed:"))
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	return int64(h.Sum64() & 0x7fffffffffffffff)
}

func Create(ctx context.Context, opts CreateOptions) (string, error) {
	if strings.TrimSpace(opts.Name) == "" {
		return "", fmt.Errorf("universe name is required")
	}
	if Slug(opts.Name) == "" {
		return "", fmt.Errorf("universe name must contain letters or digits")
	}
	seed := opts.Seed
	if seed == 0 {
		seed = SeedFromName(opts.Name)
	}
	path := DatabasePath(opts.DataDir, opts.Name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("universe %q already exists at %s", opts.Name, path)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	store, err := storage.Open(path)
	if err != nil {
		return "", err
	}
	defer store.Close()

	now := time.Now().UTC()
	u := storage.Universe{
		ID:               "uni_" + Slug(opts.Name),
		Name:             strings.TrimSpace(opts.Name),
		Seed:             seed,
		CreatedAt:        now,
		Age:              0,
		Entropy:          0.12,
		ArchiveIntegrity: 0.96,
	}
	state := storage.CivilisationState{Status: "active", Stability: 0.72, Doctrine: 0.28}
	entity := storage.Entity{
		ID:         "civ_" + Slug(opts.Name) + "_choir",
		UniverseID: u.ID,
		Kind:       "civilisation",
		Name:       initialCivilisationName(seed),
		State:      storage.EncodeCivilisationState(state),
		ValidFrom:  0,
		RecordedAt: now,
	}
	payload, _ := json.Marshal(map[string]any{
		"seed":                 seed,
		"initial_civilisation": entity.Name,
		"archive_integrity":    u.ArchiveIntegrity,
	})
	event := timeline.Event{
		UniverseID: u.ID,
		Kind:       timeline.EventUniverseCreated,
		EntityID:   entity.ID,
		ValidTime:  0,
		RecordedAt: now,
		Payload:    string(payload),
		Summary:    fmt.Sprintf("Archive node opened on %s; %s entered the first recorded horizon.", u.Name, entity.Name),
	}
	if err := store.CreateUniverse(ctx, u, entity, event); err != nil {
		return "", err
	}
	return path, nil
}

func initialCivilisationName(seed int64) string {
	names := []string{
		"Choir Republic",
		"Glass Meridian",
		"Archive Hegemony",
		"Salt Observatory",
		"Lantern Commonwealth",
		"Umbral Concord",
	}
	return names[int(seed%int64(len(names)))]
}
