// Package mcp implements the MCP (Model Context Protocol) server.
//
// It exposes two tools via stdio transport:
//
//   - list_knowledge_bases(): returns available knowledge bases
//   - ask_knowledge_base(question, effort?, knowledge_bases?): queries knowledge bases
//
// The MCP server acts as a bridge between AI agents and kb-server,
// translating MCP tool calls into Unix socket requests.
//
// Tool parameters:
//
//	ask_knowledge_base:
//	  - question: free-form natural language query (required)
//	  - effort: "low" | "medium" | "high" (default: medium)
//	  - knowledge_bases: optional list of knowledge base names to search
package mcp
