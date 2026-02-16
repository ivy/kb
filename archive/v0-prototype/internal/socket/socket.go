package socket

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

// ResolveSocketPath returns the validated Unix socket path for kb.
// It delegates to resolveSocketPath with /tmp as the fallback base directory.
func ResolveSocketPath() (string, error) {
	return resolveSocketPath("/tmp")
}

// resolveSocketPath resolves the socket path, preferring XDG_RUNTIME_DIR.
// When XDG_RUNTIME_DIR is set, it validates the directory and returns
// $XDG_RUNTIME_DIR/kb.sock. When unset, it creates <base>/kb-$UID/ with
// mode 0700, validates it, and returns <base>/kb-$UID/kb.sock.
//
// The base parameter exists only for testing with t.TempDir() and is not
// exposed publicly. Production code should use ResolveSocketPath().
//
// Per ADR-002, this function validates the directory before returning
// any socket path to ensure the socket is created in a secure location.
func resolveSocketPath(base string) (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		if err := validateDir(dir); err != nil {
			return "", fmt.Errorf("XDG_RUNTIME_DIR %q is insecure: %w", dir, err)
		}
		return filepath.Join(dir, "kb.sock"), nil
	}

	log.Printf("XDG_RUNTIME_DIR not set, using fallback directory")

	uid := os.Getuid()
	dir := filepath.Join(base, fmt.Sprintf("kb-%d", uid))

	// Create directory if it doesn't exist
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create socket directory %q: %w", dir, err)
	}

	// Validate the directory we just created (or that already existed)
	if err := validateDir(dir); err != nil {
		return "", fmt.Errorf("socket directory %q is insecure: %w", dir, err)
	}

	return filepath.Join(dir, "kb.sock"), nil
}

// validateDir verifies that path is a secure directory suitable for socket files.
// Per ADR-002, it rejects the directory if:
//   - path is a symlink (prevents symlink attacks)
//   - path is not a directory
//   - path is not owned by the current user (prevents using another user's directory)
//   - path mode is not exactly 0700 (prevents world-readable or group-readable socket directories)
func validateDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("cannot stat directory: %w", err)
	}

	// ADR-002: Reject symlinks to prevent symlink attacks where an attacker
	// replaces the directory with a symlink to a location they control.
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("path is a symlink (security violation)")
	}

	// Verify it's actually a directory
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	// ADR-002: Verify ownership to prevent using another user's directory.
	// An attacker could trick the process into using their directory by
	// manipulating environment variables or file system state.
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("cannot get file stat info")
	}
	currentUID := uint32(os.Getuid())
	if stat.Uid != currentUID {
		return fmt.Errorf("directory owned by uid %d, expected %d", stat.Uid, currentUID)
	}

	// ADR-002: Require exactly 0700 permissions to prevent other users from
	// reading the directory (which would allow them to discover and connect
	// to the socket) or writing to it (which would allow them to replace
	// the socket file with a malicious one).
	const requiredMode = 0700
	actualMode := info.Mode().Perm()
	if actualMode != requiredMode {
		return fmt.Errorf("directory mode is %04o, expected %04o", actualMode, requiredMode)
	}

	return nil
}

// ListenSocket creates a Unix domain socket listener at path.
// Per ADR-002, it validates the parent directory BEFORE removing any stale
// socket file. This ensures we never remove a file from an insecure directory,
// which could be exploited in a TOCTOU attack.
//
// If the directory validation fails, the function returns an error without
// attempting to remove the stale socket or bind, preventing the server from
// operating in an insecure configuration.
func ListenSocket(path string) (net.Listener, error) {
	dir := filepath.Dir(path)

	// ADR-002: Validate directory BEFORE any stale socket removal.
	// This ordering is critical: we must not remove files from an
	// attacker-controlled directory, even if they appear to be stale sockets.
	if err := validateDir(dir); err != nil {
		return nil, fmt.Errorf("socket directory validation failed: %w", err)
	}

	// Try to bind first. If it succeeds, there was no stale socket.
	ln, err := net.Listen("unix", path)
	if err == nil {
		return ln, nil
	}

	// If the path exists but no one is listening, it's stale — remove
	// and retry. If someone is listening, the retry will fail with
	// EADDRINUSE, which is the correct behavior.
	//
	// We've already validated the directory above, so it's safe to
	// remove the stale socket here.
	os.Remove(path)
	return net.Listen("unix", path)
}
