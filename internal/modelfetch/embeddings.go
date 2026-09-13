package modelfetch

import "github.com/ElodineOfficial/GobboNet/internal/catalog"

// The retrieval model. js/08-rag.js embeds lore and prose against it and falls
// back to weighted tags when it cannot, silently, so an install without this
// loses semantic retrieval and says nothing.
//
// Not in models.ini on purpose: that catalogue is the chat models the picker
// offers, and offering an embedder there would let someone select it as their
// chat model. It is pinned rather than fetched from the catalogue for the same
// reason launch.bat pinned it -- the hash is the only thing standing between a
// re-uploaded asset and a silently different embedding space.
//
// ⛔ Do not substitute another embedder without checking task prefixes:
// js/08-rag.js hardcodes nomic's "search_document: " / "search_query: ".
const (
	EmbeddingRepo   = "nomic-ai/nomic-embed-text-v1.5-GGUF"
	EmbeddingFile   = "nomic-embed-text-v1.5.Q8_0.gguf"
	EmbeddingSHA256 = "3e24342164b3d94991ba9692fdc0dd08e3fd7362e0aacc396a9a5c54a544c3b7"
)

// EmbeddingEntry describes the retrieval model as a catalogue entry, so the
// ordinary download path verifies its checksum like any other.
func EmbeddingEntry() catalog.Entry {
	return catalog.Entry{
		Display: "Nomic Embed Text v1.5 (retrieval)",
		Repo:    EmbeddingRepo,
		File:    EmbeddingFile,
		SizeGB:  0.146,
		SHA256:  EmbeddingSHA256,
	}
}
