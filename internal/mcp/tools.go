// internal/mcp/tools.go
package mcp

// ListKnowledgeBasesInput is the input for list_knowledge_bases.
type ListKnowledgeBasesInput struct{}

// AskKnowledgeBaseInput is the input for ask_knowledge_base.
type AskKnowledgeBaseInput struct {
	Question       string   `json:"question" jsonschema_description:"Your question"`
	Effort         string   `json:"effort,omitempty" jsonschema_description:"Search depth: low (quick lookup) / medium (balanced) / high (thorough)"`
	KnowledgeBases []string `json:"knowledge_bases,omitempty" jsonschema_description:"Limit search to specific knowledge bases by name. Omit to search all."`
}
