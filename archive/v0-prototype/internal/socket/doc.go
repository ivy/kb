// Package socket resolves and validates Unix socket paths per ADR-002.
//
// See docs/adrs/002-unix-socket-directory-security.md for security rationale.
// Every validation check exists to mitigate a specific threat modeled in that ADR.
package socket
