package universe

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
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

type CreatedUniverse struct {
	ID   string
	Name string
	Path string
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

func UniverseDir(dataDir, id string) string {
	return filepath.Join(DataDir(dataDir), "universes", strings.TrimSpace(id))
}

func DatabasePath(dataDir, id string) string {
	return filepath.Join(UniverseDir(dataDir, id), "universe.sqlite")
}

func HostKeyPath(dataDir, id string) string {
	return filepath.Join(UniverseDir(dataDir, id), "ssh_host_ed25519")
}

func OperatorKeyPath(dataDir string) string {
	return filepath.Join(DataDir(dataDir), "operator_ed25519")
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

func Create(ctx context.Context, opts CreateOptions) (CreatedUniverse, error) {
	if strings.TrimSpace(opts.Name) == "" {
		return CreatedUniverse{}, fmt.Errorf("universe name is required")
	}
	if Slug(opts.Name) == "" {
		return CreatedUniverse{}, fmt.Errorf("universe name must contain letters or digits")
	}
	seed := opts.Seed
	if seed == 0 {
		seed = SeedFromName(opts.Name)
	}
	universeID, err := NewID(rand.Reader)
	if err != nil {
		return CreatedUniverse{}, err
	}
	path := DatabasePath(opts.DataDir, universeID)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return CreatedUniverse{}, err
	}
	if _, err := os.Stat(path); err == nil {
		return CreatedUniverse{}, fmt.Errorf("universe %q already exists at %s", opts.Name, path)
	} else if !os.IsNotExist(err) {
		return CreatedUniverse{}, err
	}
	store, err := storage.Open(path)
	if err != nil {
		return CreatedUniverse{}, err
	}
	defer store.Close()

	now := time.Now().UTC()
	civilisationID, err := NewID(rand.Reader)
	if err != nil {
		return CreatedUniverse{}, err
	}
	u := storage.Universe{
		ID:               universeID,
		Name:             strings.TrimSpace(opts.Name),
		Seed:             seed,
		CreatedAt:        now,
		Age:              0,
		Entropy:          0.12,
		ArchiveIntegrity: 0.96,
	}
	state := storage.CivilisationState{Status: "active", Stability: 0.72, Doctrine: 0.28}
	entity := storage.Entity{
		ID:         civilisationID,
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
		return CreatedUniverse{}, err
	}
	return CreatedUniverse{ID: u.ID, Name: u.Name, Path: path}, nil
}

func ResolveDatabasePath(ctx context.Context, dataDir, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", fmt.Errorf("universe id or name is required")
	}
	if isPathSegment(ref) {
		path := DatabasePath(dataDir, ref)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	matches, err := findUniverses(ctx, dataDir, ref)
	if err != nil {
		return "", err
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("universe %q not found", ref)
	case 1:
		return matches[0].Path, nil
	default:
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			ids = append(ids, match.ID)
		}
		return "", fmt.Errorf("universe name %q is ambiguous; use one of these ids: %s", ref, strings.Join(ids, ", "))
	}
}

func NewID(random io.Reader) (string, error) {
	var b [16]byte
	if _, err := io.ReadFull(random, b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4],
		b[4:6],
		b[6:8],
		b[8:10],
		b[10:16]), nil
}

type locatedUniverse struct {
	ID   string
	Path string
}

func findUniverses(ctx context.Context, dataDir, ref string) ([]locatedUniverse, error) {
	paths, err := filepath.Glob(filepath.Join(DataDir(dataDir), "universes", "*", "universe.sqlite"))
	if err != nil {
		return nil, err
	}
	var matches []locatedUniverse
	for _, path := range paths {
		store, err := storage.Open(path)
		if err != nil {
			return nil, err
		}
		u, loadErr := store.LoadUniverse(ctx)
		closeErr := store.Close()
		if loadErr != nil {
			return nil, loadErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if u.ID == ref || u.Name == ref {
			matches = append(matches, locatedUniverse{ID: u.ID, Path: path})
		}
	}
	return matches, nil
}

func isPathSegment(ref string) bool {
	return ref != "." && ref != ".." && ref == filepath.Base(ref)
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
