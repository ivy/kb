package chunk

// Chunk represents a semantically meaningful piece of a document.
type Chunk struct {
	Content          string   // the chunk text
	ContentHash      []byte   // BLAKE3 hash of Content
	TokenCount       int      // exact cl100k_base token count
	HeadingHierarchy []string // e.g., ["Architecture", "Data Model", "Chunks"]
	Position         int      // 0-indexed position within the file
}
