package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ElodineOfficial/GobboNet/internal/modelfetch"
)

// cmdFetchEmbeddings downloads the retrieval model.
//
// Separate from setup because it is a network fetch this project should never
// perform unasked: "fully offline" is the promise on the tin, and an install
// that quietly pulls 146 MB on first start breaks it. The wizard will offer it;
// this is the headless door onto the same thing.
func cmdFetchEmbeddings(argv []string) error {
	fs := flag.NewFlagSet("gobbonet fetch-embeddings", flag.ContinueOnError)
	configPath := stringFlag(fs, "config", "path to config.toml")
	force := fs.Bool("force", false, "download again even if the file is already there")
	if err := fs.Parse(argv); err != nil {
		return err
	}
	cfg, err := loadConfig(*configPath)
	if err != nil {
		return err
	}

	dest := cfg.EmbedModelPath()
	if _, err := os.Stat(dest); err == nil && !*force {
		fmt.Printf(" [OK] already present: %s\n", dest)
		return nil
	}

	entry := modelfetch.EmbeddingEntry()
	fmt.Printf(" [..] %s\n      %s/%s -> %s\n", entry.Display, entry.Repo, entry.File, filepath.Dir(dest))

	dl := modelfetch.New(entry, filepath.Dir(dest), modelfetch.RequireChecksum(true))
	go dl.Run()

	last := -1.0
	for {
		st := dl.Status()
		if st.Percent-last >= 5 {
			fmt.Printf("      %.0f%% (%d/%d MiB)\n", st.Percent, st.Done>>20, st.Total>>20)
			last = st.Percent
		}
		switch st.State {
		case "done":
			fmt.Printf(" [OK] verified against the pinned checksum: %s\n", dest)
			fmt.Println("      Restart GobboNet to start the embedding server.")
			return nil
		case "error":
			return fmt.Errorf("download failed: %s", st.Message)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
