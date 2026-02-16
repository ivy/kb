# How to Debug the kb Pipeline

## Test kb-server independently

Start the server:

```bash
go run ./cmd/kb-server
```

List knowledge bases:

```bash
curl --unix-socket /tmp/kb.sock \
  -H "Content-Type: application/json" \
  -d '{}' \
  http://localhost/kb.v1.KnowledgeBaseService/List
```

Query with a specific effort level:

```bash
curl --unix-socket /tmp/kb.sock \
  -H "Content-Type: application/json" \
  -d '{"question": "test query", "effort": "EFFORT_HIGH"}' \
  http://localhost/kb.v1.KnowledgeBaseService/Query
```

Use `buf curl` for typed output (uses Connect protocol over HTTP/1.1):

```bash
buf curl --unix-socket /tmp/kb.sock \
  --schema proto \
  -d '{}' \
  http://localhost/kb.v1.KnowledgeBaseService/List
```

## Test kb-mcp independently

Send JSON-RPC on stdin (kb-server must be running). The MCP protocol requires
an `initialized` notification after the `initialize` handshake, and stdin must
stay open long enough for the server to respond:

```bash
(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'; sleep 1) | go run ./cmd/kb-mcp 2>/dev/null
```

Call a tool directly:

```bash
(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ask_knowledge_base","arguments":{"question":"test","effort":"high"}}}'; sleep 1) | go run ./cmd/kb-mcp 2>/dev/null
```

kb-mcp logs go to stderr. Redirect stderr to see them:

```bash
... | go run ./cmd/kb-mcp 2>&1
```

## Fix a stale socket file

If kb-server crashes without cleaning up, the socket file remains. kb-server handles this automatically on startup, but if you see `EADDRINUSE`, another instance is already running:

```bash
# Check if something is listening
lsof /tmp/kb.sock

# If nothing is listening, remove the stale file
rm /tmp/kb.sock
```

## Regenerate protobuf code

After editing `proto/kb/v1/kb.proto`:

```bash
go generate ./...
```

Verify nothing is stale:

```bash
go generate ./... && git diff --exit-code gen/
```

If `buf` is not found, ensure build tools are installed with `mise install`.
