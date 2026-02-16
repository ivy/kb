# Auth Architecture

How KB authenticates callers and authorizes access. Read [query.md](query.md) for the caller interface, [admin.md](admin.md) for the admin interface, this doc for who's allowed to use them.

## Approach

KB uses [tsnet](https://pkg.go.dev/tailscale.com/tsnet) to join the tailnet as a node. No separate auth system, no API keys, no tokens. Identity comes from Tailscale — every request arrives from a known node with a known user.

```go
s := &tsnet.Server{Hostname: "kb"}
defer s.Close()

ln, _ := s.Listen("tcp", ":80")
lc, _ := s.LocalClient()
```

Every incoming request can be identified via `WhoIs`:

```go
who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
// who.UserProfile — the Tailscale user
// who.Node — the device making the request
// who.CapMap — granted application capabilities
```

## Application Capabilities

Network access alone isn't sufficient — KB needs to distinguish between callers (who can ask questions) and admins (who can manage sources and trigger indexing). [Tailscale application capabilities](https://tailscale.com/docs/features/access-control/grants/grants-app-capabilities) handle this at the policy layer.

### Capability Definitions

| Capability | Grants |
|-----------|--------|
| `homestead.local/cap/kb/ask` | Query the KB agent (`POST /v1/ask`, `POST /v1/sessions/:id/ask`) |
| `homestead.local/cap/kb/admin` | Manage sources, trigger reindex, view status (CLI or future admin API) |

### Policy Example

```json
{
  "grants": [
    {
      "src": ["tag:agent"],
      "dst": ["tag:kb"],
      "app": {
        "homestead.local/cap/kb/ask": [{}]
      }
    },
    {
      "src": ["autogroup:admin"],
      "dst": ["tag:kb"],
      "app": {
        "homestead.local/cap/kb/ask": [{}],
        "homestead.local/cap/kb/admin": [{}]
      }
    }
  ]
}
```

Agents tagged `tag:agent` can ask questions. Tailnet admins can ask questions and manage KB. The webhook endpoint is separate — GitLab's node needs network access to the webhook port but no application capabilities.

### Enforcement

The server checks capabilities on each request:

```go
who, _ := lc.WhoIs(r.Context(), r.RemoteAddr)
if !hasCapability(who.CapMap, "homestead.local/cap/kb/ask") {
    http.Error(w, "not authorized", 403)
    return
}
```

The CLI (`kb sources`, `kb reindex`, etc.) connects to PostgreSQL directly, so authorization there is database-level — only nodes that can reach the database can administrate. In practice, this is the same node running KB.

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| tsnet over standalone auth | Tailscale identity | No API keys to manage. Identity is the network. Zero-trust by default. |
| Application capabilities over network ACLs | `homestead.local/cap/kb/*` | Network access alone can't distinguish ask vs. admin. Capabilities add application-layer authorization without custom auth code. |
| Two capabilities | ask + admin | Minimal surface. Callers ask, admins manage. No fine-grained permissions needed for a single-homestead MVP. |
| Webhook auth via network | GitLab node reaches webhook port | System hooks don't support custom auth headers well. Network-level access from the GitLab node is sufficient. |

## Deferred

- **Per-source access control** — zone scoping (grove agents can't see hearth data) is a future concern. MVP trusts all callers equally.
- **Capability-scoped search results** — filtering results based on caller capabilities. Requires integrating access control into the query pipeline.
