---
status: accepted
date: 2026-01-31
---

# Secure Unix socket path with a private runtime directory

## Context and Problem Statement

`kb-server` listens on a Unix domain socket for RPC from `kb-mcp`. Knowledge
bases may contain sensitive personal or organizational information. When
`XDG_RUNTIME_DIR` is unset, the current fallback places the socket directly
in `/tmp/kb.sock`. This has two security problems:

1. **No access control.** Any local user can connect to the socket and query
   or exfiltrate knowledge base contents.
2. **Pre-bind / symlink attacks.** A malicious local user can create or
   symlink `/tmp/kb.sock` before the server starts, intercepting requests
   or causing the server to bind to an attacker-controlled path.

Even when `XDG_RUNTIME_DIR` is set, the system guarantee of `0700` may not
hold on misconfigured hosts. Given the sensitivity of the data served, the
server MUST NOT trust the environment blindly.

## Decision Drivers

* Knowledge bases may contain sensitive information -- defense in depth is
  required
* Socket MUST NOT be accessible to other local users under any circumstance
* Server MUST refuse to start rather than operate with degraded security
* Follow established Unix conventions (ssh-agent, tmux, systemd)
* Comply with the XDG Base Directory Specification for `XDG_RUNTIME_DIR`

## Considered Options

* **Bare socket in `/tmp`** (status quo)
* **Private per-user directory with mandatory validation** (hardened
  tmux/ssh-agent pattern)
* **Require `XDG_RUNTIME_DIR`; refuse to start without it**

## Decision Outcome

Chosen option: "Private per-user directory with mandatory validation",
because it eliminates the identified attack surface, enforces security
invariants regardless of environment configuration, and follows proven Unix
conventions.

### Implementation

**Directory validation** (applies to ALL paths, including `XDG_RUNTIME_DIR`):

Before binding, the server MUST validate the socket's parent directory:

1. `Lstat` the directory (not `Stat`) to detect symlinks.
2. It MUST be a real directory (not a symlink).
3. It MUST be owned by the current user (`uid` match).
4. It MUST have mode `0700` (no group or world access).
5. If any check fails, the server MUST refuse to start with a clear error
   message identifying which check failed and how to fix it.

**When `XDG_RUNTIME_DIR` is set:**

1. Use `$XDG_RUNTIME_DIR/kb.sock`.
2. Apply the directory validation above. Do NOT assume the system set
   permissions correctly.

**When `XDG_RUNTIME_DIR` is unset:**

1. Log a warning that `XDG_RUNTIME_DIR` is not set (per the spec's fallback
   guidance).
2. Create `/tmp/kb-$UID/` with mode `0700`.
3. Apply the directory validation above.
4. Place the socket inside: `/tmp/kb-$UID/kb.sock`.

**Socket file cleanup:**

The existing stale-socket removal logic (`os.Remove` then retry) MUST only
execute after directory validation passes, to avoid removing an
attacker-placed file in an unvalidated directory.

### Consequences

* Good, because sensitive knowledge base data is protected from other local
  users regardless of environment configuration
* Good, because the server fails closed -- it refuses to start rather than
  operating insecurely
* Good, because validation on every startup detects directory tampering
  between runs
* Good, because it follows the same pattern as tmux and ssh-agent, which
  users and operators already understand
* Good, because it complies with XDG Base Directory Specification fallback
  guidance
* Bad, because the server will refuse to start if the directory has been
  tampered with, requiring manual cleanup -- this is intentional
* Bad, because a misconfigured `XDG_RUNTIME_DIR` that previously worked
  silently will now cause a startup failure -- this surfaces real problems

### Confirmation

* Unit test: `listenSocket` MUST refuse to bind when the parent directory
  has wrong ownership, wrong mode, or is a symlink
* Unit test: verify the fallback path is `/tmp/kb-$UID/kb.sock`
* Unit test: verify `XDG_RUNTIME_DIR` path is also validated (not
  trusted blindly)
* Unit test: stale-socket removal MUST NOT execute before directory
  validation
* Integration: `kb-server` logs a warning when `XDG_RUNTIME_DIR` is unset

## Pros and Cons of the Options

### Bare socket in `/tmp` (status quo)

* Good, because zero implementation effort
* Bad, because any local user can connect and access knowledge base data
* Bad, because vulnerable to pre-bind and symlink attacks
* Bad, because violates the principle of least privilege
* Bad, because no path to safe multi-user operation

### Private per-user directory with mandatory validation

* Good, because proven pattern used by tmux, ssh-agent, and others
* Good, because no configuration required
* Good, because validates security invariants on every startup, including
  for `XDG_RUNTIME_DIR`
* Good, because fails closed on any validation failure
* Neutral, because adds ~30-40 lines of directory management and validation
  code
* Bad, because directory may accumulate across reboots if not cleaned up
  (harmless empty directory)

### Require `XDG_RUNTIME_DIR`

* Good, because simplest implementation (just error out)
* Good, because delegates directory security to the system
* Bad, because macOS does not set `XDG_RUNTIME_DIR` by default
* Bad, because forces users to configure their environment before first use
* Bad, because still trusts the environment without validation

## More Information

* [XDG Base Directory Specification -- `XDG_RUNTIME_DIR`](https://specifications.freedesktop.org/basedir/latest/)
* tmux creates `/tmp/tmux-$UID/` with mode `0700` and validates ownership
  on every startup; refuses to use a directory it doesn't own
* ssh-agent uses `mkdtemp("/tmp/ssh-XXXXXXXXXX")` with mode `0700`
