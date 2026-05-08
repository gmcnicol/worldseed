package daemon

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

type ConnectOptions struct {
	Addr   string
	Stdin  *os.File
	Stdout *os.File
	Stderr io.Writer
}

func Connect(ctx context.Context, opts ConnectOptions) error {
	if opts.Addr == "" {
		opts.Addr = "127.0.0.1:27411"
	}
	if opts.Stdin == nil {
		opts.Stdin = os.Stdin
	}
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}
	config := &ssh.ClientConfig{
		User:            "operator",
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", opts.Addr)
	if err != nil {
		return err
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, opts.Addr, config)
	if err != nil {
		return err
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	width, height, err := term.GetSize(int(opts.Stdout.Fd()))
	if err != nil {
		width, height = 96, 32
	}
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := session.RequestPty("xterm-256color", height, width, modes); err != nil {
		return err
	}
	oldState, err := term.MakeRaw(int(opts.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(opts.Stdin.Fd()), oldState)
	}
	session.Stdin = opts.Stdin
	session.Stdout = opts.Stdout
	session.Stderr = opts.Stderr
	if err := session.Shell(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	select {
	case <-ctx.Done():
		_ = session.Close()
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("ssh session closed: %w", err)
		}
		return nil
	}
}
