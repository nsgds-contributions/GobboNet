package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The retrieval-model paths resolve like every other path key: against the
// config's own folder, with ~ as the home folder. A shortcut starts GobboNet in
// the install folder and the Linux launcher in its prefix, so a raw relative
// value pointed somewhere different on every launcher.
func TestEmbedPathsResolveLikeTheOtherPaths(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path := filepath.Join(dir, "config.toml")
	body := "llm_url = \"http://x:1\"\n" +
		"embed_model = \"embeddings/custom.gguf\"\n" +
		"embed_exe = \"~/llama.cpp/llama-server\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	// Run from somewhere else, as every launcher does.
	t.Chdir(t.TempDir())

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "embeddings", "custom.gguf"); cfg.EmbedModel != want || cfg.EmbedModelPath() != want {
		t.Errorf("embed_model: got %q (EmbedModelPath %q), want %q", cfg.EmbedModel, cfg.EmbedModelPath(), want)
	}
	if want := filepath.Join(home, "llama.cpp", "llama-server"); cfg.EmbedExe != want {
		t.Errorf("embed_exe: got %q, want %q", cfg.EmbedExe, want)
	}
}

// Unset stays unset, so EmbedModelPath still falls back to the data folder.
func TestUnsetEmbedPathsStayUnset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("llm_url = \"http://x:1\"\ndata_dir = \"./data\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EmbedModel != "" || cfg.EmbedExe != "" {
		t.Errorf("unset keys came back set: embed_model=%q embed_exe=%q", cfg.EmbedModel, cfg.EmbedExe)
	}
	if want := filepath.Join(dir, "data", "embeddings", "nomic-embed-text-v1.5.Q8_0.gguf"); cfg.EmbedModelPath() != want {
		t.Errorf("EmbedModelPath: got %q, want %q", cfg.EmbedModelPath(), want)
	}
}
