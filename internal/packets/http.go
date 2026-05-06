package packets

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/worldseed/worldseed/internal/universe"
)

func SocketPath(rootDir, universe string) string {
	return filepath.Join(rootDir, universe, "worldseedd.sock")
}

func StartServer(ctx context.Context, rootDir, universeID string, svc *universe.Service) error {
	sock := SocketPath(rootDir, universeID)
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		s, err := svc.Snapshot(context.Background(), universeID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_ = json.NewEncoder(w).Encode(s)
	})
	mux.HandleFunc("/interventions/preserve_archive", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method", 405)
			return
		}
		if err := svc.PreserveArchive(context.Background(), universeID); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.WriteHeader(202)
	})
	s := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		c, _ := context.WithTimeout(context.Background(), 2*time.Second)
		_ = s.Shutdown(c)
	}()
	err = s.Serve(ln)
	_ = os.Remove(sock)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}
