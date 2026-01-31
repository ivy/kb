// Package mcp implements the MCP (Model Context Protocol) server.
//
// It exposes a single tool, knowledge_query, via stdio transport.
// The MCP server acts as a bridge between AI agents and kb-server,
// translating MCP tool calls into Unix socket requests.
//
// Tool definition:
//
//	knowledge_query(question, effort?, vaults?)
//	  - question: free-form natural language query
//	  - effort: "low" | "medium" | "high" (default: medium)
//	  - vaults: optional list of vault names to search
package mcp
