package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// execCmd builds a fresh rootCmd, injects the given args plus --config pointing
// at the test database, captures stdout (os.Stdout), and returns it.
// CLI commands use fmt.Println which writes to os.Stdout, so we redirect it.
func execCmd(configPath string, args ...string) (string, error) {
	// Redirect os.Stdout to a pipe so we capture fmt.Print* output.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", fmt.Errorf("creating pipe: %w", err)
	}
	os.Stdout = w

	cmd := rootCmd()
	cmd.SetArgs(append(args, "--config", configPath))
	execErr := cmd.ExecuteContext(context.Background())

	// Close writer and restore stdout before reading.
	_ = w.Close()
	os.Stdout = origStdout

	buf, _ := io.ReadAll(r)
	_ = r.Close()

	return string(buf), execErr
}

var _ = Describe("CLI commands", Label("integration"), func() {
	var (
		ctx        context.Context
		container  *postgres.PostgresContainer
		configPath string
	)

	BeforeEach(func() {
		ctx = context.Background()

		var err error
		container, err = postgres.Run(ctx, "pgvector/pgvector:pg17",
			postgres.WithDatabase("kb_test"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(30*time.Second),
			),
		)
		Expect(err).NotTo(HaveOccurred())

		connStr, err := container.ConnectionString(ctx, "sslmode=disable")
		Expect(err).NotTo(HaveOccurred())

		// Write a minimal config file pointing at the test DB.
		f, err := os.CreateTemp("", "kb-test-*.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = fmt.Fprintf(f, "database:\n  url: %s\n", connStr)
		Expect(err).NotTo(HaveOccurred())
		Expect(f.Close()).To(Succeed())
		configPath = f.Name()
	})

	AfterEach(func() {
		if configPath != "" {
			_ = os.Remove(configPath)
		}
		if container != nil {
			Expect(container.Terminate(ctx)).To(Succeed())
		}
	})

	Describe("kb setup", func() {
		It("runs migrations and prints 'setup complete'", func() {
			out, err := execCmd(configPath, "setup")
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring("setup complete"))
		})

		It("is idempotent on repeated runs", func() {
			_, err := execCmd(configPath, "setup")
			Expect(err).NotTo(HaveOccurred())

			out, err := execCmd(configPath, "setup")
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring("setup complete"))
		})
	})

	Describe("kb sources", func() {
		// Run setup first so the schema exists.
		JustBeforeEach(func() {
			_, err := execCmd(configPath, "setup")
			Expect(err).NotTo(HaveOccurred())
		})

		Context("with no sources configured", func() {
			It("prints 'no sources configured'", func() {
				out, err := execCmd(configPath, "sources", "list")
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(ContainSubstring("no sources configured"))
			})
		})

		Context("after adding a source", func() {
			It("prints 'added source: <url>'", func() {
				out, err := execCmd(configPath, "sources", "add", "https://gitlab.com/test/repo")
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(ContainSubstring("added source: https://gitlab.com/test/repo"))
			})

			It("appears in sources list", func() {
				_, err := execCmd(configPath, "sources", "add", "https://gitlab.com/test/repo")
				Expect(err).NotTo(HaveOccurred())

				out, err := execCmd(configPath, "sources", "list")
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(ContainSubstring("https://gitlab.com/test/repo (gitlab)"))
			})
		})
	})
})
