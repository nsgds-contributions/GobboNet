package main

import (
	"github.com/ElodineOfficial/GobboNet/internal/config"
	"os"
	"path/filepath"
	"testing"
)

// The hazard is a CPU-only llama.cpp archive bundled by mistake: same filenames
// as the GPU one minus a library, so the install works, offers the same models,
// and runs all of them on the processor with nothing to say so.
func TestGpuBackendsBeside(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		want  []string
	}{
		// The four pinned archives, verbatim. ggml-rpc ships in ALL of them and
		// is not an accelerator: a denylist that only skipped base and cpu
		// reported it as a GPU backend and let the CPU-only build pass.
		{"win cpu-only", []string{"llama-server.exe", "ggml-base.dll", "ggml-rpc.dll", "ggml-rpc-server.exe", "ggml-cpu-alderlake.dll", "ggml-cpu-cannonlake.dll", "ggml-cpu-cascadelake.dll", "ggml-cpu-cooperlake.dll", "ggml-cpu-haswell.dll", "ggml-cpu-icelake.dll", "ggml-cpu-ivybridge.dll", "ggml-cpu-piledriver.dll", "ggml-cpu-sandybridge.dll", "ggml-cpu-sapphirerapids.dll", "ggml-cpu-skylakex.dll", "ggml-cpu-sse42.dll", "ggml-cpu-x64.dll", "ggml-cpu-zen4.dll"}, nil},
		{"win vulkan", []string{"llama-server.exe", "ggml-vulkan.dll", "ggml-base.dll", "ggml-rpc.dll", "ggml-rpc-server.exe", "ggml-cpu-alderlake.dll", "ggml-cpu-cannonlake.dll", "ggml-cpu-cascadelake.dll", "ggml-cpu-cooperlake.dll", "ggml-cpu-haswell.dll", "ggml-cpu-icelake.dll", "ggml-cpu-ivybridge.dll", "ggml-cpu-piledriver.dll", "ggml-cpu-sandybridge.dll", "ggml-cpu-sapphirerapids.dll", "ggml-cpu-skylakex.dll", "ggml-cpu-sse42.dll", "ggml-cpu-x64.dll", "ggml-cpu-zen4.dll"}, []string{"ggml-vulkan.dll"}},
		{"linux cpu-only", []string{"llama-server", "libggml-base.so", "libggml-base.so.0", "libggml-base.so.0.20.0", "libggml-rpc.so", "libggml-cpu-haswell.so", "libggml-cpu-icelake.so"}, nil},
		{"linux vulkan", []string{"llama-server", "libggml-vulkan.so", "libggml-base.so", "libggml-base.so.0", "libggml-base.so.0.20.0", "libggml-rpc.so", "libggml-cpu-haswell.so", "libggml-cpu-icelake.so"}, []string{"libggml-vulkan.so"}},
		{"cuda", []string{"llama-server", "libggml-base.so", "libggml-cuda.so"}, []string{"libggml-cuda.so"}},
		{"blas is not an accelerator", []string{"llama-server", "libggml-base.so", "libggml-blas.so"}, nil},
		{"cpu variants are not backends", []string{"llama-server", "libggml-cpu-haswell.so"}, nil},
		{"unrelated files ignored", []string{"llama-server", "README.md", "ggml.h", "vulkan-1.dll"}, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			for _, f := range c.files {
				if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := gpuBackendsBeside(filepath.Join(dir, c.files[0]))
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i] != c.want[i] {
					t.Errorf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

func TestGpuBackendsBesideMissingDirIsNotFatal(t *testing.T) {
	if _, err := gpuBackendsBeside(filepath.Join(t.TempDir(), "gone", "llama-server")); err == nil {
		t.Fatal("a missing directory must report an error so the caller stays quiet")
	}
}

// The uninstaller resolves paths the same way every other command does. It did
// not: it passed the bare --config flag to Load, so with no flag nothing was
// found and every path fell back to a default. Three defaults are right, and
// the fourth is model_dir — which the Windows installer always moves — so
// "delete the models" deleted nothing and said it found none.
func TestUninstallDiscoversTheConfig(t *testing.T) {
	dir := t.TempDir()
	cfgDir := filepath.Join(dir, "config", "gobbonet")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	elsewhere := filepath.Join(dir, "somewhere-else", "models")
	body := "data_dir = \"" + filepath.ToSlash(filepath.Join(dir, "data")) + "\"\n" +
		"model_dir = \"" + filepath.ToSlash(elsewhere) + "\"\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))

	found, _ := config.Discover("")
	cfg, err := config.Load(found)
	if err != nil {
		t.Fatalf("discovery found %q, which did not load: %v", found, err)
	}
	if filepath.ToSlash(cfg.ModelDir) != filepath.ToSlash(elsewhere) {
		t.Errorf("model_dir = %q, want %q — uninstall would clear the wrong directory",
			cfg.ModelDir, elsewhere)
	}
}
