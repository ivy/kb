package chunk

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/pkoukk/tiktoken-go"
	"github.com/zeebo/blake3"
)

const (
	maxTokens = 4096
	encoding  = "cl100k_base"
)

// ChunkMarkdown splits markdown content into semantically meaningful chunks.
func ChunkMarkdown(content []byte) ([]Chunk, error) {
	enc, err := tiktoken.GetEncoding(encoding)
	if err != nil {
		return nil, err
	}

	text := stripFrontmatter(content)
	if len(strings.TrimSpace(text)) == 0 {
		return nil, nil
	}

	sections := splitOnHeadings(text)
	var chunks []Chunk
	pos := 0

	for _, sec := range sections {
		content := strings.TrimSpace(sec.content)
		if content == "" {
			continue
		}

		tokens := enc.Encode(content, nil, nil)
		if len(tokens) <= maxTokens {
			chunks = append(chunks, makeChunk(content, len(tokens), sec.hierarchy, pos))
			pos++
		} else {
			subChunks := splitOnParagraphs(content, sec.hierarchy, enc, pos)
			chunks = append(chunks, subChunks...)
			pos += len(subChunks)
		}
	}

	return chunks, nil
}

type section struct {
	hierarchy []string
	content   string
}

// stripFrontmatter removes YAML frontmatter delimited by --- at the document start.
func stripFrontmatter(content []byte) string {
	if !bytes.HasPrefix(content, []byte("---")) {
		return string(content)
	}

	rest := content[3:]
	idx := bytes.Index(rest, []byte("\n---"))
	if idx < 0 {
		return string(content)
	}

	// Skip past the closing --- and any trailing newline.
	after := rest[idx+4:]
	if len(after) > 0 && after[0] == '\n' {
		after = after[1:]
	}
	return string(after)
}

// splitOnHeadings splits markdown text into sections based on ATX headings.
func splitOnHeadings(text string) []section {
	lines := strings.Split(text, "\n")
	var sections []section

	// Track the current heading at each level (1-6).
	headings := make([]string, 7) // index 0 unused
	var currentContent strings.Builder
	currentLevel := 0

	flush := func() {
		c := currentContent.String()
		if strings.TrimSpace(c) != "" || currentLevel > 0 {
			hier := buildHierarchy(headings, currentLevel)
			sections = append(sections, section{
				hierarchy: hier,
				content:   c,
			})
		}
		currentContent.Reset()
	}

	for _, line := range lines {
		level, title := parseHeading(line)
		if level > 0 {
			flush()
			// Clear all headings at this level and below.
			for i := level; i <= 6; i++ {
				headings[i] = ""
			}
			headings[level] = title
			currentLevel = level
			continue
		}
		currentContent.WriteString(line)
		currentContent.WriteByte('\n')
	}
	flush()

	return sections
}

// parseHeading returns the heading level (1-6) and title, or 0 if not a heading.
func parseHeading(line string) (int, string) {
	if !utf8.ValidString(line) {
		return 0, ""
	}
	trimmed := strings.TrimLeft(line, " ")
	if len(trimmed) == 0 || trimmed[0] != '#' {
		return 0, ""
	}

	level := 0
	for _, c := range trimmed {
		if c == '#' {
			level++
		} else {
			break
		}
	}
	if level > 6 {
		return 0, ""
	}
	if level >= len(trimmed) {
		// Heading with no title text (e.g., just "##").
		return level, ""
	}
	if trimmed[level] != ' ' {
		return 0, "" // e.g., "#tag" is not a heading
	}

	title := strings.TrimSpace(trimmed[level+1:])
	return level, title
}

// buildHierarchy returns the heading ancestry for the given level.
func buildHierarchy(headings []string, level int) []string {
	var hier []string
	for i := 1; i <= level; i++ {
		if headings[i] != "" {
			hier = append(hier, headings[i])
		}
	}
	return hier
}

// splitOnParagraphs splits oversized content on paragraph boundaries (\n\n).
func splitOnParagraphs(content string, hierarchy []string, enc *tiktoken.Tiktoken, startPos int) []Chunk {
	paragraphs := strings.Split(content, "\n\n")
	var chunks []Chunk
	var current strings.Builder
	currentTokens := 0
	pos := startPos

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		paraTokens := len(enc.Encode(para, nil, nil))

		if current.Len() > 0 {
			// Check if adding this paragraph would exceed the limit.
			combined := current.String() + "\n\n" + para
			combinedTokens := len(enc.Encode(combined, nil, nil))
			if combinedTokens > maxTokens {
				// Flush current buffer.
				text := current.String()
				chunks = append(chunks, makeChunk(text, currentTokens, hierarchy, pos))
				pos++
				current.Reset()
				current.WriteString(para)
				currentTokens = paraTokens
				continue
			}
			current.WriteString("\n\n")
			current.WriteString(para)
			currentTokens = combinedTokens
		} else {
			current.WriteString(para)
			currentTokens = paraTokens
		}
	}

	if current.Len() > 0 {
		text := current.String()
		chunks = append(chunks, makeChunk(text, currentTokens, hierarchy, pos))
	}

	return chunks
}

func makeChunk(content string, tokenCount int, hierarchy []string, position int) Chunk {
	h := blake3.Sum256([]byte(content))
	// Copy hierarchy to avoid aliasing.
	hier := make([]string, len(hierarchy))
	copy(hier, hierarchy)
	return Chunk{
		Content:          content,
		ContentHash:      h[:],
		TokenCount:       tokenCount,
		HeadingHierarchy: hier,
		Position:         position,
	}
}
