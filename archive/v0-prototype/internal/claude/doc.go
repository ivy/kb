// Package claude provides a Go wrapper for headless Claude Code.
//
// It invokes Claude Code CLI in non-interactive mode to synthesize
// answers from search results. The package handles:
//   - Custom system prompts for knowledge retrieval
//   - Working directory control (vault root)
//   - JSON output parsing
//   - Effort level configuration
package claude
