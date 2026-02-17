package chunk_test

import (
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkoukk/tiktoken-go"
	"github.com/zeebo/blake3"

	"github.com/ivy/kb/internal/chunk"
)

func testdataPath(name string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", name)
}

func readFixture(name string) []byte {
	data, err := os.ReadFile(testdataPath(name))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return data
}

var _ = Describe("Markdown chunking", func() {
	Describe("determinism", func() {
		It("produces identical chunks and hashes for the same input across calls", func() {
			content := readFixture("multi-section.md")
			chunks1, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())
			chunks2, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())

			Expect(chunks1).To(HaveLen(len(chunks2)))
			for i := range chunks1 {
				Expect(chunks1[i].Content).To(Equal(chunks2[i].Content))
				Expect(chunks1[i].ContentHash).To(Equal(chunks2[i].ContentHash))
				Expect(chunks1[i].TokenCount).To(Equal(chunks2[i].TokenCount))
				Expect(chunks1[i].Position).To(Equal(chunks2[i].Position))
			}
		})

		It("produces identical chunks regardless of surrounding file content", func() {
			content := readFixture("multi-section.md")
			chunks1, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())

			// Append a new section — existing chunks must not change.
			extended := append(content, []byte("\n## New Section\n\nBrand new content.\n")...)
			chunks2, err := chunk.ChunkMarkdown(extended)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(chunks2)).To(BeNumerically(">", len(chunks1)))
			for i := range chunks1 {
				Expect(chunks2[i].Content).To(Equal(chunks1[i].Content))
				Expect(chunks2[i].ContentHash).To(Equal(chunks1[i].ContentHash))
			}
		})
	})

	Describe("document structure", func() {
		Context("when the document has a single section", func() {
			var chunks []chunk.Chunk

			BeforeEach(func() {
				content := readFixture("simple.md")
				var err error
				chunks, err = chunk.ChunkMarkdown(content)
				Expect(err).NotTo(HaveOccurred())
			})

			It("produces one chunk containing the section content", func() {
				Expect(chunks).To(HaveLen(1))
				Expect(chunks[0].Content).To(ContainSubstring("simple document with a single section"))
			})

			It("records the heading as the hierarchy root", func() {
				Expect(chunks[0].HeadingHierarchy).To(Equal([]string{"Simple Document"}))
			})
		})

		Context("when the document has multiple sections", func() {
			var chunks []chunk.Chunk

			BeforeEach(func() {
				content := readFixture("multi-section.md")
				var err error
				chunks, err = chunk.ChunkMarkdown(content)
				Expect(err).NotTo(HaveOccurred())
			})

			It("produces one chunk per heading-delimited section", func() {
				Expect(chunks).To(HaveLen(3))
			})

			It("assigns position indices in document order", func() {
				for i, c := range chunks {
					Expect(c.Position).To(Equal(i))
				}
			})

			It("tracks each section's heading hierarchy independently", func() {
				Expect(chunks[0].HeadingHierarchy).To(Equal([]string{"Multi-Section Document", "Section One"}))
				Expect(chunks[1].HeadingHierarchy).To(Equal([]string{"Multi-Section Document", "Section Two"}))
				Expect(chunks[2].HeadingHierarchy).To(Equal([]string{"Multi-Section Document", "Section Three"}))
			})
		})

		Context("when headings are nested (H1 > H2 > H3)", func() {
			It("builds the full ancestor path for each chunk", func() {
				content := readFixture("nested-headings.md")
				chunks, err := chunk.ChunkMarkdown(content)
				Expect(err).NotTo(HaveOccurred())

				Expect(chunks).To(HaveLen(4))
				Expect(chunks[0].HeadingHierarchy).To(Equal([]string{"Top Level"}))
				Expect(chunks[1].HeadingHierarchy).To(Equal([]string{"Top Level", "Second Level"}))
				Expect(chunks[2].HeadingHierarchy).To(Equal([]string{"Top Level", "Second Level", "Third Level"}))
				Expect(chunks[3].HeadingHierarchy).To(Equal([]string{"Top Level", "Another Second Level"}))
			})
		})
	})

	Describe("content hashing", func() {
		It("computes a BLAKE3 hash of the chunk content", func() {
			content := readFixture("simple.md")
			chunks, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(chunks).To(HaveLen(1))

			expected := blake3.Sum256([]byte(chunks[0].Content))
			Expect(chunks[0].ContentHash).To(Equal(expected[:]))
		})

		It("produces different hashes for different content", func() {
			content := readFixture("multi-section.md")
			chunks, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(chunks).To(HaveLen(3))

			Expect(chunks[0].ContentHash).NotTo(Equal(chunks[1].ContentHash))
			Expect(chunks[1].ContentHash).NotTo(Equal(chunks[2].ContentHash))
		})

		It("produces the same hash for identical content in different files", func() {
			// Build two different documents that share a section with identical content.
			doc1 := []byte("# Doc One\n\nIntro one.\n\n## Shared\n\nIdentical content here.\n")
			doc2 := []byte("# Doc Two\n\nIntro two.\n\n## Shared\n\nIdentical content here.\n")

			chunks1, err := chunk.ChunkMarkdown(doc1)
			Expect(err).NotTo(HaveOccurred())
			chunks2, err := chunk.ChunkMarkdown(doc2)
			Expect(err).NotTo(HaveOccurred())

			// The "Shared" section has the same content, so hashes must match.
			Expect(chunks1[1].ContentHash).To(Equal(chunks2[1].ContentHash))
		})
	})

	Describe("token counting", func() {
		It("reports the exact cl100k_base token count for each chunk", func() {
			content := readFixture("simple.md")
			chunks, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(chunks).To(HaveLen(1))

			enc, err := tiktoken.GetEncoding("cl100k_base")
			Expect(err).NotTo(HaveOccurred())
			expected := len(enc.Encode(chunks[0].Content, nil, nil))
			Expect(chunks[0].TokenCount).To(Equal(expected))
		})

		It("never produces a chunk exceeding 4096 tokens", func() {
			content := readFixture("oversized-section.md")
			chunks, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())

			for _, c := range chunks {
				Expect(c.TokenCount).To(BeNumerically("<=", 4096))
			}
		})
	})

	Describe("oversized sections", func() {
		Context("when a section exceeds the 4096 token limit", func() {
			var chunks []chunk.Chunk

			BeforeEach(func() {
				content := readFixture("oversized-section.md")
				var err error
				chunks, err = chunk.ChunkMarkdown(content)
				Expect(err).NotTo(HaveOccurred())
			})

			It("splits on paragraph boundaries", func() {
				// An oversized section must produce multiple chunks.
				Expect(len(chunks)).To(BeNumerically(">=", 2))
			})

			It("preserves the heading hierarchy on every sub-chunk", func() {
				for _, c := range chunks {
					Expect(c.HeadingHierarchy).To(Equal([]string{"Oversized Document", "Very Large Section"}))
				}
			})

			It("keeps every resulting chunk within the token limit", func() {
				for _, c := range chunks {
					Expect(c.TokenCount).To(BeNumerically("<=", 4096))
				}
			})
		})
	})

	Describe("edge cases", func() {
		It("strips YAML frontmatter before chunking", func() {
			content := readFixture("frontmatter.md")
			chunks, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(chunks).To(HaveLen(1))
			Expect(chunks[0].Content).NotTo(ContainSubstring("---"))
			Expect(chunks[0].Content).NotTo(ContainSubstring("tags"))
			Expect(chunks[0].Content).To(ContainSubstring("frontmatter that should be skipped"))
		})

		It("returns zero chunks for an empty document", func() {
			content := readFixture("empty.md")
			chunks, err := chunk.ChunkMarkdown(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(chunks).To(BeEmpty())
		})
	})
})
