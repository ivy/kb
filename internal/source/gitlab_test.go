package source_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ivy/kb/internal/source"
)

var ctx = context.Background()

// buildArchive creates a tar.gz archive in memory with the given files under
// the specified root directory prefix (mimicking GitLab's archive format).
func buildArchive(rootDir string, files map[string][]byte) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Write the root directory entry.
	_ = tw.WriteHeader(&tar.Header{
		Name:     rootDir + "/",
		Typeflag: tar.TypeDir,
		Mode:     0o755,
	})

	for path, content := range files {
		_ = tw.WriteHeader(&tar.Header{
			Name:     rootDir + "/" + path,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
			Mode:     0o644,
		})
		_, _ = tw.Write(content)
	}

	_ = tw.Close()
	_ = gw.Close()
	return buf.Bytes()
}

var _ = Describe("GitLab source adapter", func() {
	var (
		server       *httptest.Server
		receivedReqs []*http.Request
	)

	AfterEach(func() {
		if server != nil {
			server.Close()
		}
		receivedReqs = nil
	})

	Describe("archive extraction", func() {
		It("returns only Markdown files from the archive", func() {
			archive := buildArchive("project-main-abc123", map[string][]byte{
				"README.md":      []byte("# README"),
				"docs/guide.md":  []byte("# Guide"),
				"main.go":        []byte("package main"),
				"config.yaml":    []byte("key: value"),
				"docs/notes.txt": []byte("some notes"),
			})

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedReqs = append(receivedReqs, r)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(archive)
			}))

			result, err := source.FetchGitLab(ctx, server.URL+"/user/project", "test-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Files).To(HaveLen(2))
			Expect(result.Files).To(HaveKey("README.md"))
			Expect(result.Files).To(HaveKey("docs/guide.md"))
			Expect(result.Files).NotTo(HaveKey("main.go"))
			Expect(result.Files).NotTo(HaveKey("config.yaml"))
		})

		It("uses relative paths with the archive root prefix stripped", func() {
			archive := buildArchive("project-main-abc123", map[string][]byte{
				"docs/architecture/overview.md": []byte("# Overview"),
				"README.md":                     []byte("# Root"),
			})

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(archive)
			}))

			result, err := source.FetchGitLab(ctx, server.URL+"/user/project", "test-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Files).To(HaveKey("docs/architecture/overview.md"))
			Expect(result.Files).To(HaveKey("README.md"))
			// No entry should contain the root prefix.
			for path := range result.Files {
				Expect(path).NotTo(HavePrefix("project-main-abc123"))
			}
		})

		It("captures the commit ref from the archive", func() {
			archive := buildArchive("project-main-abc123", map[string][]byte{
				"README.md": []byte("# Hello"),
			})

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(archive)
			}))

			result, err := source.FetchGitLab(ctx, server.URL+"/user/project", "test-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Ref).To(Equal("abc123"))
		})
	})

	Describe("authentication", func() {
		It("sends the configured token via the PRIVATE-TOKEN header", func() {
			archive := buildArchive("project-main-abc123", map[string][]byte{
				"README.md": []byte("# Hello"),
			})

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedReqs = append(receivedReqs, r)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(archive)
			}))

			_, err := source.FetchGitLab(ctx, server.URL+"/user/project", "my-secret-token")
			Expect(err).NotTo(HaveOccurred())
			Expect(receivedReqs).To(HaveLen(1))
			Expect(receivedReqs[0].Header.Get("PRIVATE-TOKEN")).To(Equal("my-secret-token"))
		})
	})

	Describe("error handling", func() {
		Context("when the GitLab API returns a non-200 status", func() {
			It("returns an error containing the status code and URL", func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))

				_, err := source.FetchGitLab(ctx, server.URL+"/user/project", "token")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("404"))
				Expect(err.Error()).To(ContainSubstring("/api/v4/projects/"))
			})
		})

		Context("when the archive is malformed", func() {
			It("returns an error without panicking", func() {
				server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("this is not a gzip archive"))
				}))

				result, err := source.FetchGitLab(ctx, server.URL+"/user/project", "token")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("decompressing"))
				Expect(result).To(BeNil())
			})
		})
	})
})
