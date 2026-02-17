package source

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

// FetchGitLab downloads and extracts Markdown files from a GitLab repo archive.
// Uses the repository archive API (/api/v4/projects/:id/repository/archive.tar.gz).
// Satisfies the Fetcher function type.
func FetchGitLab(ctx context.Context, sourceURL, token string) (*Result, error) {
	parsed, err := url.Parse(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("source: invalid URL %q: %w", sourceURL, err)
	}

	// Derive project path from URL: e.g. /user/project → user%2Fproject
	projectPath := strings.TrimPrefix(parsed.Path, "/")
	projectPath = strings.TrimSuffix(projectPath, "/")
	if projectPath == "" {
		return nil, fmt.Errorf("source: no project path in URL %q", sourceURL)
	}
	encodedPath := url.PathEscape(projectPath)

	archiveURL := fmt.Sprintf("%s://%s/api/v4/projects/%s/repository/archive.tar.gz",
		parsed.Scheme, parsed.Host, encodedPath)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return nil, fmt.Errorf("source: creating request: %w", err)
	}
	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("source: fetching archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("source: GET %s returned %d", archiveURL, resp.StatusCode)
	}

	return extractArchive(resp.Body)
}

// extractArchive reads a tar.gz stream and returns Markdown files with the
// archive's top-level directory prefix stripped.
func extractArchive(r io.Reader) (*Result, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("source: decompressing archive: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	result := &Result{
		Files: make(map[string][]byte),
	}

	var rootPrefix string

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("source: reading tar entry: %w", err)
		}

		name := filepath.Clean(hdr.Name)

		// Determine the root prefix from the first entry (GitLab archives
		// always have a top-level directory like project-ref-sha/).
		if rootPrefix == "" {
			parts := strings.SplitN(name, string(filepath.Separator), 2)
			rootPrefix = parts[0]
			// Extract ref from the root directory name.
			// Format: project-name-ref-shortsha or project-ref
			result.Ref = extractRef(rootPrefix)
		}

		// Strip the root prefix.
		rel, err := filepath.Rel(rootPrefix, name)
		if err != nil || rel == "." {
			continue
		}

		// Skip directories.
		if hdr.Typeflag == tar.TypeDir {
			continue
		}

		// Skip non-Markdown files.
		if !strings.HasSuffix(strings.ToLower(rel), ".md") {
			continue
		}

		// Skip .git/ contents.
		if strings.HasPrefix(rel, ".git"+string(filepath.Separator)) || rel == ".git" {
			continue
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("source: reading %s: %w", rel, err)
		}

		result.Files[rel] = content
	}

	return result, nil
}

// extractRef parses the commit ref from a GitLab archive root directory name.
// GitLab names the root directory as: project-name-ref-shortsha
// We extract the last component after the project name as the ref.
func extractRef(dirName string) string {
	// GitLab archive root directories look like:
	//   project-main-abc123def
	//   my-project-v1.0.0-abc123def
	// The last hyphen-separated segment is the short SHA.
	// Everything between the project name and the SHA is the ref.
	// Since we don't know the project name length, we take the last
	// segment as a reasonable ref identifier.
	parts := strings.Split(dirName, "-")
	if len(parts) < 2 {
		return dirName
	}
	// Return the last part as the short SHA / ref identifier.
	return parts[len(parts)-1]
}
