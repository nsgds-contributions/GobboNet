package supervisor

// The embedding server, which nothing started before this.
//
// launch.bat spawned it on the batch path; the Go server only ever proxied to
// EmbedURL. So a Go install had semantic retrieval switched off with no error
// anywhere, because js/08-rag.js is degrade-safe and falls back to weighted-tag
// retrieval. A card written as prose simply stopped being retrievable.
//
// It is deliberately not a second Supervisor: there is no swapping, no crash
// backoff and no status feed. Embeddings are optional by construction, so the
// failure mode here is "log it and carry on", never "refuse to serve".

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type EmbedOptions struct {
	Exe     string
	Model   string
	URL     string
	LogFile string
	Out     io.Writer
}

type Embedder struct {
	opts EmbedOptions
	mu   sync.Mutex
	cmd  *exec.Cmd
	pgid int
}

func NewEmbedder(opts EmbedOptions) *Embedder {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	return &Embedder{opts: opts}
}

// Start spawns the embedding server unless it is unnecessary or impossible.
// Every "no" is a normal outcome and returns nil; only a spawn that was
// attempted and failed returns an error, and even that is not fatal to serving.
func (e *Embedder) Start() error {
	if e.opts.Exe == "" || e.opts.URL == "" {
		fmt.Fprintf(e.opts.Out, " [*]  embeddings: no engine to run one with\n")
		fmt.Fprintf(e.opts.Out, "      Retrieval falls back to weighted tags; chat is unaffected.\n")
		return nil
	}
	hostPort, err := hostPortOf(e.opts.URL)
	if err != nil {
		return fmt.Errorf("embed_url: %w", err)
	}

	// Somebody else's, and leaving it alone is the point: a sidecar container or
	// a hand-started llama-server is a supported way to provide this.
	if answering(hostPort) {
		fmt.Fprintf(e.opts.Out, " [OK] embeddings: already answering on %s\n", hostPort)
		return nil
	}

	if _, err := os.Stat(e.opts.Model); err != nil {
		fmt.Fprintf(e.opts.Out, " [*]  embeddings: no model at %s\n", e.opts.Model)
		fmt.Fprintf(e.opts.Out, "      Retrieval falls back to weighted tags; chat is unaffected.\n")
		return nil
	}

	host, port, _ := net.SplitHostPort(hostPort)
	args := []string{
		"--model", e.opts.Model,
		"--host", host,
		"--port", port,
		"--embeddings",
		// Required by nomic/e5-style embedders. Without it llama.cpp uses the
		// model default, which for this checkpoint is not mean pooling, and the
		// vectors come back subtly wrong rather than absent.
		"--pooling", "mean",
		"--ctx-size", "2048",
		"-ngl", "0",
	}

	cmd := exec.Command(e.opts.Exe, args...)
	configureProcessGroup(cmd)
	if e.opts.LogFile != "" {
		if f, err := os.OpenFile(e.opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			cmd.Stdout, cmd.Stderr = f, f
			defer f.Close()
		}
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("embedding server: %w", err)
	}
	if err := superviseTree(cmd); err != nil {
		fmt.Fprintf(e.opts.Out, " [!]  embeddings: not attached to the kill job: %v\n", err)
	}

	e.mu.Lock()
	e.cmd, e.pgid = cmd, processGroupID(cmd)
	e.mu.Unlock()

	go func() {
		_ = cmd.Wait()
		e.mu.Lock()
		if e.cmd == cmd {
			e.cmd = nil
		}
		e.mu.Unlock()
	}()

	// Readiness is reported asynchronously: a spawn that exits immediately -- a
	// corrupt GGUF, a bind clash, a missing DLL -- used to print OK and leave the
	// only trace in a log file, which is the silent switch-off this whole type
	// exists to end. Serving must not wait for it either.
	fmt.Fprintf(e.opts.Out, " [..] embeddings: starting %s on %s\n", filepath.Base(e.opts.Model), hostPort)
	go e.report(hostPort)
	return nil
}

func (e *Embedder) report(hostPort string) {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if answering(hostPort) {
			fmt.Fprintf(e.opts.Out, " [OK] embeddings: ready on %s\n", hostPort)
			return
		}
		if !e.Running() {
			break
		}
		time.Sleep(time.Second)
	}
	fmt.Fprintf(e.opts.Out, " [!]  embeddings: never became ready on %s\n", hostPort)
	if e.opts.LogFile != "" {
		fmt.Fprintf(e.opts.Out, "      See %s. Retrieval falls back to weighted tags.\n", e.opts.LogFile)
	}
}

// Stop is nil-safe: the serve path calls it from a signal handler that cannot
// know whether embeddings were ever enabled.
func (e *Embedder) Stop() {
	if e == nil {
		return
	}
	e.mu.Lock()
	cmd, pgid := e.cmd, e.pgid
	e.cmd = nil
	e.mu.Unlock()
	if cmd == nil {
		return
	}
	// Short grace, then force -- and no grace at all if the polite stop was
	// refused, which is what happens on every clean exit on Windows. Same rule
	// as Supervisor.stop(); there is no state here to lose either way.
	if err := terminateGroup(pgid, false); err != nil || !waitGroupGone(pgid, 2*time.Second) {
		_ = terminateGroup(pgid, true)
	}
}

func (e *Embedder) Running() bool {
	if e == nil {
		return false
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cmd != nil
}

func hostPortOf(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("no host in %q", raw)
	}
	if u.Port() == "" {
		return net.JoinHostPort(u.Hostname(), "80"), nil
	}
	return u.Host, nil
}

// answering reports whether llama.cpp owns this port. 200 is ready; 503 is
// loading and still means occupied, so we must not spawn a second instance into
// a bind failure. Anything else is some other service, and treating it as ours
// would hand the retriever non-embedding responses while reporting success.
// A refused connection is the expected answer on a free port, not an error.
func answering(hostPort string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+hostPort+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	return resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable
}
