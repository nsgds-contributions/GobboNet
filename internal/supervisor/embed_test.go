package supervisor

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// Forcing a layer count disables llama.cpp's own fit, which then aborts and
// fails to allocate the KV cache on any card too small for the whole model.
func TestGpuLayersArg(t *testing.T) {
	cases := map[int]string{-1: "auto", 0: "0", 20: "20", 99: "99"}
	for in, want := range cases {
		if got := gpuLayersArg(in); got != want {
			t.Errorf("gpuLayersArg(%d)=%q want %q", in, got, want)
		}
	}
}
