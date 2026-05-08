package daemon

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gmcnicol/worldseed/internal/sim"
	"github.com/gmcnicol/worldseed/internal/storage"
	"github.com/gmcnicol/worldseed/internal/tui"
	"github.com/gmcnicol/worldseed/internal/universe"
	"golang.org/x/crypto/ssh"
)

type StartOptions struct {
	DataDir      string
	UniverseName string
	Addr         string
	TickInterval time.Duration
	Logger       *slog.Logger
}

type Server struct {
	opts        StartOptions
	store       *storage.Store
	engine      *sim.Engine
	started     time.Time
	hostKeyPath string
	logger      *slog.Logger
}

func Start(ctx context.Context, opts StartOptions) error {
	if strings.TrimSpace(opts.UniverseName) == "" {
		return fmt.Errorf("universe name is required")
	}
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:27411"
	}
	if opts.TickInterval <= 0 {
		opts.TickInterval = 5 * time.Second
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	dbPath, err := universe.ResolveDatabasePath(ctx, opts.DataDir, opts.UniverseName)
	if err != nil {
		return err
	}
	store, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	server := &Server{
		opts:        opts,
		store:       store,
		engine:      sim.NewEngine(store),
		started:     time.Now().UTC(),
		hostKeyPath: filepath.Join(filepath.Dir(dbPath), "ssh_host_ed25519"),
		logger:      opts.Logger.With("universe", opts.UniverseName),
	}
	return server.run(ctx)
}

func (s *Server) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	signer, err := loadOrCreateHostSigner(s.hostKeyPath)
	if err != nil {
		return err
	}
	config := &ssh.ServerConfig{
		PublicKeyCallback: acceptClientKey,
		ServerVersion:     "SSH-2.0-worldseedd",
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", s.opts.Addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	s.logger.Info("worldseedd listening", "addr", listener.Addr().String(), "tick_interval", s.opts.TickInterval)

	var wg sync.WaitGroup
	errs := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.simulationLoop(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs <- s.acceptLoop(ctx, listener, config)
	}()

	select {
	case <-ctx.Done():
		s.logger.Info("worldseedd shutting down", "reason", ctx.Err())
	case err := <-errs:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			cancel()
			_ = listener.Close()
			wg.Wait()
			return err
		}
	}
	_ = listener.Close()
	wg.Wait()
	return nil
}

func (s *Server) simulationLoop(ctx context.Context) {
	ticker := time.NewTicker(s.opts.TickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := s.engine.Tick(ctx, 1)
			if err != nil {
				s.logger.Error("simulation tick failed", "error", err)
				continue
			}
			for _, event := range result.Events {
				s.logger.Info("timeline event", "age", event.ValidTime, "kind", event.Kind, "summary", event.Summary)
			}
		}
	}
}

func (s *Server) acceptLoop(ctx context.Context, listener net.Listener, config *ssh.ServerConfig) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return ctx.Err()
		default:
		}
		go s.handleConn(conn, config)
	}
}

func (s *Server) handleConn(conn net.Conn, config *ssh.ServerConfig) {
	sshConn, channels, requests, err := ssh.NewServerConn(conn, config)
	if err != nil {
		s.logger.Warn("ssh handshake failed", "remote", conn.RemoteAddr().String(), "error", err)
		return
	}
	defer sshConn.Close()
	s.recordClientKey(sshConn)
	go ssh.DiscardRequests(requests)
	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(ssh.UnknownChannelType, "session channels only")
			continue
		}
		ch, reqs, err := channel.Accept()
		if err != nil {
			s.logger.Warn("ssh channel accept failed", "error", err)
			continue
		}
		go s.handleSession(ch, reqs)
	}
}

func (s *Server) handleSession(ch ssh.Channel, requests <-chan *ssh.Request) {
	defer ch.Close()
	var once sync.Once
	runDashboard := func() {
		status := uint32(0)
		program := tea.NewProgram(tui.New(s.store, s.started), tea.WithInput(ch), tea.WithOutput(ch))
		if _, err := program.Run(); err != nil {
			s.logger.Warn("dashboard session failed", "error", err)
			status = 1
		}
		sendExitStatus(ch, status)
	}
	for req := range requests {
		switch req.Type {
		case "pty-req":
			_ = req.Reply(true, nil)
		case "window-change":
			_ = req.Reply(false, nil)
		case "shell":
			_ = req.Reply(true, nil)
			once.Do(runDashboard)
			return
		case "exec":
			command := parseExecCommand(req.Payload)
			if command != "" && command != "dashboard" {
				_, _ = io.WriteString(ch.Stderr(), "unknown worldseed ssh command\n")
				_ = req.Reply(false, nil)
				return
			}
			_ = req.Reply(true, nil)
			once.Do(runDashboard)
			return
		default:
			_ = req.Reply(false, nil)
		}
	}
}

func acceptClientKey(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	return &ssh.Permissions{
		Extensions: map[string]string{
			"username":    conn.User(),
			"fingerprint": ssh.FingerprintSHA256(key),
			"public_key":  strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
		},
	}, nil
}

func (s *Server) recordClientKey(conn *ssh.ServerConn) {
	perms := conn.Permissions
	if perms == nil {
		s.logger.Warn("ssh client connected without key permissions", "remote", conn.RemoteAddr().String())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	u, err := s.store.LoadUniverse(ctx)
	if err != nil {
		s.logger.Warn("ssh client key association failed", "remote", conn.RemoteAddr().String(), "error", err)
		return
	}
	now := time.Now().UTC()
	key := storage.ClientKey{
		UniverseID:  u.ID,
		Username:    perms.Extensions["username"],
		Fingerprint: perms.Extensions["fingerprint"],
		PublicKey:   perms.Extensions["public_key"],
		FirstSeenAt: now,
		LastSeenAt:  now,
		RemoteAddr:  conn.RemoteAddr().String(),
	}
	if err := s.store.RecordClientKey(ctx, key); err != nil {
		s.logger.Warn("ssh client key association failed", "remote", conn.RemoteAddr().String(), "fingerprint", key.Fingerprint, "error", err)
		return
	}
	s.logger.Info("ssh client connected", "remote", conn.RemoteAddr().String(), "fingerprint", key.Fingerprint)
}

func loadOrCreateHostSigner(path string) (ssh.Signer, error) {
	body, err := os.ReadFile(path)
	if err == nil {
		return ssh.ParsePrivateKey(body)
	}
	if !os.IsNotExist(err) {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(key, "worldseed universe host key")
	if err != nil {
		return nil, err
	}
	body = pem.EncodeToMemory(block)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(body)
}

func parseExecCommand(payload []byte) string {
	if len(payload) < 4 {
		return ""
	}
	size := binary.BigEndian.Uint32(payload[:4])
	if int(size) > len(payload)-4 {
		return ""
	}
	return strings.TrimSpace(string(payload[4 : 4+size]))
}

func sendExitStatus(ch ssh.Channel, status uint32) {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, status)
	_, _ = ch.SendRequest("exit-status", false, payload)
}
