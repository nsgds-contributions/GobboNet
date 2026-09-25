package main

import (
	"os"
	"path/filepath"
	"testing"
)

// uninstallFixture is an install whose config points model_dir and embed_model
// wherever the test says, with a file in each place that must survive.
func uninstallFixture(t *testing.T, modelDir, embedModel func(home string) string) (home, dataDir string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("GOBBONET_CONFIG", "")
	dataDir = filepath.Join(home, "data", "gobbonet")

	body := "data_dir = \"" + filepath.ToSlash(dataDir) + "\"\n"
	if modelDir != nil {
		body += "model_dir = \"" + filepath.ToSlash(modelDir(home)) + "\"\n"
	}
	if embedModel != nil {
		body += "embed_model = \"" + filepath.ToSlash(embedModel(home)) + "\"\n"
	}
	writeFile(t, filepath.Join(home, "config", "gobbonet", "config.toml"), body)
	writeFile(t, filepath.Join(dataDir, "state.json"), "{}")
	writeFile(t, filepath.Join(dataDir, "models", "chat.gguf"), "x")
	writeFile(t, filepath.Join(dataDir, "embeddings", "nomic-embed-text-v1.5.Q8_0.gguf"), "x")
	return home, dataDir
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// embed_model names a file. Its folder is the user's -- Downloads, a models
// folder shared with other tools -- and removing "the models" used to delete
// that whole folder, unattended, from the Windows uninstaller's checkbox.
func TestUninstallRemovesACustomRetrievalModelAsAFile(t *testing.T) {
	for _, argv := range [][]string{
		{"--yes", "--models-only"},
		{"--yes", "--remove-models"},
	} {
		t.Run(argv[1], func(t *testing.T) {
			home, dataDir := uninstallFixture(t, nil, func(home string) string {
				return filepath.Join(home, "Downloads", "nomic-embed-text-v1.5.Q8_0.gguf")
			})
			shared := filepath.Join(home, "Downloads")
			writeFile(t, filepath.Join(shared, "nomic-embed-text-v1.5.Q8_0.gguf"), "x")
			writeFile(t, filepath.Join(shared, "tax-return.pdf"), "keep")

			if err := cmdUninstall(argv); err != nil {
				t.Fatal(err)
			}
			if !exists(filepath.Join(shared, "tax-return.pdf")) {
				t.Fatal("an unrelated file beside the retrieval model was deleted")
			}
			if exists(filepath.Join(shared, "nomic-embed-text-v1.5.Q8_0.gguf")) {
				t.Error("the retrieval model the config names was kept")
			}
			for _, d := range []string{"models", "embeddings"} {
				if exists(filepath.Join(dataDir, d)) {
					t.Errorf("GobboNet's own %s folder was kept", d)
				}
			}
		})
	}
}

// "Models only" promises to leave settings and conversations. A retrieval model
// kept directly in the data folder used to make its folder -- the data folder --
// the thing removed.
func TestModelsOnlyKeepsConversationsBesideARetrievalModel(t *testing.T) {
	_, dataDir := uninstallFixture(t, nil, func(home string) string {
		return filepath.Join(home, "data", "gobbonet", "nomic-embed-text-v1.5.Q8_0.gguf")
	})
	writeFile(t, filepath.Join(dataDir, "nomic-embed-text-v1.5.Q8_0.gguf"), "x")

	if err := cmdUninstall([]string{"--yes", "--models-only"}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dataDir, "state.json")) {
		t.Fatal("--models-only deleted the conversations")
	}
	if exists(filepath.Join(dataDir, "nomic-embed-text-v1.5.Q8_0.gguf")) {
		t.Error("the retrieval model was kept")
	}
}

// model_dir is the user's to point anywhere: at a folder shared with other
// tools, or one holding the data folder itself. Removing the models takes the
// .gguf files GobboNet would list and any half-finished download -- never the
// folder's other contents, and never a subfolder.
func TestUninstallTakesModelFilesNotTheFolder(t *testing.T) {
	for _, argv := range [][]string{
		{"--yes", "--models-only"},
		{"--yes", "--remove-models"},
	} {
		t.Run(argv[1], func(t *testing.T) {
			home, _ := uninstallFixture(t, func(home string) string {
				return filepath.Join(home, "AI-Models")
			}, nil)
			shared := filepath.Join(home, "AI-Models")
			writeFile(t, filepath.Join(shared, "gobbonet-chat.gguf"), "x")
			writeFile(t, filepath.Join(shared, "Half-Fetched.GGUF.part"), "x")
			writeFile(t, filepath.Join(shared, "lmstudio", "other-tool.gguf"), "keep")
			writeFile(t, filepath.Join(shared, "notes.txt"), "keep")

			if err := cmdUninstall(argv); err != nil {
				t.Fatal(err)
			}
			for _, gone := range []string{"gobbonet-chat.gguf", "Half-Fetched.GGUF.part"} {
				if exists(filepath.Join(shared, gone)) {
					t.Errorf("%s was kept", gone)
				}
			}
			for _, kept := range []string{"notes.txt", filepath.Join("lmstudio", "other-tool.gguf")} {
				if !exists(filepath.Join(shared, kept)) {
					t.Errorf("%s was deleted with the models", kept)
				}
			}
		})
	}
}

// A model_dir that holds the data folder loses its models and nothing else.
func TestModelsFolderHoldingTheDataKeepsTheData(t *testing.T) {
	home, dataDir := uninstallFixture(t, func(home string) string {
		return filepath.Join(home, "data")
	}, nil)
	writeFile(t, filepath.Join(home, "data", "chat.gguf"), "x")
	writeFile(t, filepath.Join(home, "data", "other-app", "keep.txt"), "keep")

	if err := cmdUninstall([]string{"--yes", "--models-only"}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(dataDir, "state.json")) || !exists(filepath.Join(home, "data", "other-app", "keep.txt")) {
		t.Fatal("removing the models took the data folder's other contents")
	}
	if exists(filepath.Join(home, "data", "chat.gguf")) {
		t.Error("the model in model_dir was kept")
	}
}

// Without a readable config nobody knows where the models are. Guessing the
// defaults cleared the wrong place and exited 0, so the Windows uninstaller's
// own fallback for its models folder never ran.
func TestModelsOnlyRefusesAnUnreadableConfig(t *testing.T) {
	home, dataDir := uninstallFixture(t, func(home string) string {
		return filepath.Join(home, "elsewhere-models")
	}, nil)
	writeFile(t, filepath.Join(home, "elsewhere-models", "chat.gguf"), "x")
	cfg := filepath.Join(home, "config", "gobbonet", "config.toml")
	f, err := os.OpenFile(cfg, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("listen_prot = 1\n")
	f.Close()

	if err := cmdUninstall([]string{"--yes", "--models-only"}); err == nil {
		t.Fatal("an unreadable config was accepted, so the caller cannot tell nothing was cleared")
	}
	for _, p := range []string{
		filepath.Join(home, "elsewhere-models", "chat.gguf"),
		filepath.Join(dataDir, "models", "chat.gguf"),
		filepath.Join(dataDir, "embeddings", "nomic-embed-text-v1.5.Q8_0.gguf"),
	} {
		if !exists(p) {
			t.Errorf("removed %s while refusing", p)
		}
	}
}

// embed_model is removed as a file only when it is a model. Anything else it
// names is reported and left, not listed as going and then silently kept.
func TestANonModelEmbedModelIsLeftAlone(t *testing.T) {
	home, _ := uninstallFixture(t, nil, func(home string) string {
		return filepath.Join(home, "notes", "readme.txt")
	})
	writeFile(t, filepath.Join(home, "notes", "readme.txt"), "keep")
	if err := cmdUninstall([]string{"--yes", "--models-only"}); err != nil {
		t.Fatal(err)
	}
	if !exists(filepath.Join(home, "notes", "readme.txt")) {
		t.Fatal("a non-.gguf embed_model was deleted")
	}
}
