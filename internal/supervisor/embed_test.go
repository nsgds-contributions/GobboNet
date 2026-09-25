package supervisor

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// answering decides whether to spawn at all, so a wrong yes hands the retriever
// a service that does not do embeddings while reporting success, and a wrong no
// spawns a second instance into a bind failure.
func TestAnsweringOnlyAcceptsLlamaHealth(t *testing.T) {
	cases := []struct {
		name string
		code int
		want bool
	}{
		{"ready", http.StatusOK, true},
		{"loading, still ours", http.StatusServiceUnavailable, true},
		{"some other service", http.StatusNotFound, false},
		{"unauthenticated proxy", http.StatusUnauthorized, false},
		{"broken upstream", http.StatusBadGateway, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("probed %s, want /health", r.URL.Path)
				}
				w.WriteHeader(c.code)
			}))
			defer srv.Close()
			if got := answering(strings.TrimPrefix(srv.URL, "http://")); got != c.want {
				t.Errorf("answering()=%v want %v", got, c.want)
			}
		})
	}
}

func TestAnsweringFreePortIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	if answering(addr) {
		t.Fatal("a closed port must not read as answering")
	}
}

// Every reason not to spawn is a normal outcome, and each must say so: silence
// is the failure this type exists to end.
func TestStartDeclinesAudibly(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "model.gguf")
	if err := os.WriteFile(model, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	busy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer busy.Close()

	cases := []struct {
		name string
		opts EmbedOptions
		want string
	}{
		{"no engine", EmbedOptions{URL: "http://127.0.0.1:1"}, "no engine"},
		{"no url", EmbedOptions{Exe: "/nonexistent", Model: model}, "no embed_url"},
		// Remote mode: no local engine, but a sidecar already serves embed_url.
		{"remote sidecar, no engine", EmbedOptions{URL: busy.URL}, "already answering"},
		{"no model", EmbedOptions{Exe: "/nonexistent", Model: filepath.Join(dir, "absent.gguf"), URL: "http://127.0.0.1:1"}, "no model at"},
		{"already answering", EmbedOptions{Exe: "/nonexistent", Model: model, URL: busy.URL}, "already answering"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			c.opts.Out = &out
			e := NewEmbedder(c.opts)
			if err := e.Start(); err != nil {
				t.Fatalf("declining must not be an error: %v", err)
			}
			if e.Running() {
				t.Fatal("nothing should have been spawned")
			}
			if !strings.Contains(out.String(), c.want) {
				t.Errorf("output %q does not mention %q", out.String(), c.want)
			}
		})
	}
}

func TestStopAndRunningAreNilSafe(t *testing.T) {
	var e *Embedder
	e.Stop()
	if e.Running() {
		t.Fatal("nil embedder must not report running")
	}
}

func TestHostPortOf(t *testing.T) {
	cases := map[string]string{
		"http://127.0.0.1:11436":  "127.0.0.1:11436",
		"http://nomic-embed:8080": "nomic-embed:8080",
		"http://example.com":      "example.com:80",
	}
	for in, want := range cases {
		got, err := hostPortOf(in)
		if err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if got != want {
			t.Errorf("%s -> %s, want %s", in, got, want)
		}
	}
	if _, err := hostPortOf("::not a url"); err == nil {
		t.Error("a garbage embed_url must be an error, not a silent skip")
	}
}

// Without the batch pair an embedding server answers 500 to anything over 512
// tokens, and retrieval stops without a word. Pinned because nothing else would
// notice it going missing again.
func TestEmbedArgsCarryTheBatchSize(t *testing.T) {
	args := strings.Join(embedArgs("m.gguf", "127.0.0.1", "11436"), " ")
	for _, want := range []string{
		"--embeddings", "--pooling mean", "--ctx-size 2048",
		"--batch-size 2048", "--ubatch-size 2048", "-ngl 0", "--no-op-offload",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("embedding server args %q lack %q", args, want)
		}
	}
}

// A shutdown can arrive while Start is still deciding whether to spawn. Once
// Stop has run, Start must not leave a child behind for nobody to stop.
func TestStartAfterStopSpawnsNothing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell script as the engine")
	}
	dir := t.TempDir()
	model := filepath.Join(dir, "model.gguf")
	exe := filepath.Join(dir, "fake-llama-server")
	if err := os.WriteFile(model, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	opts := EmbedOptions{Exe: exe, Model: model, URL: "http://127.0.0.1:1", Out: io.Discard}

	e := NewEmbedder(opts)
	e.Stop()
	if err := e.Start(); err != nil {
		t.Fatal(err)
	}
	if e.Running() {
		e.Stop()
		t.Fatal("Start after Stop spawned an embedding server")
	}

	// Control: the same options do spawn when nothing stopped it first.
	c := NewEmbedder(opts)
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	defer c.Stop()
	if !c.Running() {
		t.Fatal("control: Start did not spawn, so the check above proves nothing")
	}
}
