package agent_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/ivy/kb/internal/agent"
	"github.com/ivy/kb/internal/config"
)

var _ = Describe("MRKLAgent", func() {
	Context("with an unsupported provider", func() {
		It("returns an error", func() {
			ag := agent.New(nil, nil, nil, config.Agent{
				Provider: "unsupported",
				APIKey:   "test",
			})
			_, err := ag.Ask(context.Background(), "test question", nil, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported"))
		})
	})

	Context("with an empty provider", func() {
		It("returns an error", func() {
			ag := agent.New(nil, nil, nil, config.Agent{
				APIKey: "test",
			})
			_, err := ag.Ask(context.Background(), "test question", nil, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported"))
		})
	})
})
