package socket

import (
	"os"
	"path/filepath"
	"testing"
)

// TestValidateDir_ValidDirectory verifies that a properly configured directory
// (mode 0700, correct owner, real directory) passes validation.
// Per ADR-002, this is the happy path for a secure socket directory.
func TestValidateDir_ValidDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	if err := validateDir(dir); err != nil {
		t.Errorf("validateDir() failed for valid directory: %v", err)
	}
}

// TestValidateDir_RejectsWrongMode verifies that directories with mode 0755
// are rejected to prevent world-readable socket directories.
// Per ADR-002, world-readable directories allow other users to discover
// and potentially connect to the socket.
func TestValidateDir_RejectsWrongMode(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	err := validateDir(dir)
	if err == nil {
		t.Fatal("validateDir() should reject directory with mode 0755")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestValidateDir_RejectsSymlink verifies that symlinks to valid directories
// are rejected to prevent symlink attacks.
// Per ADR-002, an attacker could replace a directory with a symlink to a
// location they control, then intercept socket traffic.
func TestValidateDir_RejectsSymlink(t *testing.T) {
	realDir := t.TempDir()
	if err := os.Chmod(realDir, 0700); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	symlinkPath := filepath.Join(t.TempDir(), "symlink")
	if err := os.Symlink(realDir, symlinkPath); err != nil {
		t.Fatalf("symlink creation failed: %v", err)
	}

	err := validateDir(symlinkPath)
	if err == nil {
		t.Fatal("validateDir() should reject symlink to valid directory")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestValidateDir_RejectsWrongOwner verifies that directories owned by
// other users are rejected.
// Per ADR-002, this prevents the process from trusting another user's
// directory, which that user could manipulate.
//
// This test probes /private/var/root (macOS) or /root (Linux) as a directory
// owned by root. It skips if the path is unavailable or if running as root.
func TestValidateDir_RejectsWrongOwner(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("skipping owner validation test when running as root")
	}

	// Try macOS path first, then Linux
	rootDir := "/private/var/root"
	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		rootDir = "/root"
	}

	// Check if we can stat it (might not have permission)
	if _, err := os.Lstat(rootDir); err != nil {
		t.Skipf("cannot access %s: %v", rootDir, err)
	}

	err := validateDir(rootDir)
	if err == nil {
		t.Fatalf("validateDir() should reject directory owned by root")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestValidateDir_RejectsNonDirectory verifies that regular files are rejected.
// Per ADR-002, only directories can securely contain socket files.
func TestValidateDir_RejectsNonDirectory(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("test"), 0600); err != nil {
		t.Fatalf("file creation failed: %v", err)
	}

	err := validateDir(file)
	if err == nil {
		t.Fatal("validateDir() should reject regular file")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestResolveSocketPath_XDGSet verifies that when XDG_RUNTIME_DIR is set
// to a valid directory, ResolveSocketPath returns $XDG_RUNTIME_DIR/kb.sock
// after validating the directory.
// Per ADR-002, XDG_RUNTIME_DIR is preferred but not trusted blindly.
func TestResolveSocketPath_XDGSet(t *testing.T) {
	xdgDir := t.TempDir()
	if err := os.Chmod(xdgDir, 0700); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	t.Setenv("XDG_RUNTIME_DIR", xdgDir)

	path, err := ResolveSocketPath()
	if err != nil {
		t.Fatalf("ResolveSocketPath() failed: %v", err)
	}

	expected := filepath.Join(xdgDir, "kb.sock")
	if path != expected {
		t.Errorf("got path %q, want %q", path, expected)
	}
}

// TestResolveSocketPath_XDGUnset verifies that when XDG_RUNTIME_DIR is unset,
// resolveSocketPath creates a fallback directory <base>/kb-$UID/ with mode 0700,
// validates it, and returns <base>/kb-$UID/kb.sock.
// Per ADR-002, the fallback must be as secure as the XDG path.
//
// This test uses the unexported resolveSocketPath(base) to avoid writing to
// real /tmp during tests.
func TestResolveSocketPath_XDGUnset(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "")

	base := t.TempDir()
	path, err := resolveSocketPath(base)
	if err != nil {
		t.Fatalf("resolveSocketPath() failed: %v", err)
	}

	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if err != nil {
		t.Fatalf("directory doesn't exist: %v", err)
	}

	if info.Mode().Perm() != 0700 {
		t.Errorf("directory mode is %04o, want 0700", info.Mode().Perm())
	}

	if filepath.Base(path) != "kb.sock" {
		t.Errorf("socket filename is %q, want %q", filepath.Base(path), "kb.sock")
	}
}

// TestResolveSocketPath_XDGInvalid verifies that an insecure XDG_RUNTIME_DIR
// is rejected rather than being trusted blindly.
// Per ADR-002, even XDG_RUNTIME_DIR must meet security requirements.
func TestResolveSocketPath_XDGInvalid(t *testing.T) {
	xdgDir := t.TempDir()
	// Make it world-readable (insecure)
	if err := os.Chmod(xdgDir, 0755); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	t.Setenv("XDG_RUNTIME_DIR", xdgDir)

	_, err := ResolveSocketPath()
	if err == nil {
		t.Fatal("ResolveSocketPath() should reject insecure XDG_RUNTIME_DIR")
	}
	t.Logf("correctly rejected: %v", err)
}

// TestListenSocket_StaleRemoval verifies that a stale socket file is removed
// and the listener binds successfully.
// This is the normal case when restarting the server after an unclean shutdown.
func TestListenSocket_StaleRemoval(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	socketPath := filepath.Join(dir, "test.sock")

	// Create a stale socket file
	if err := os.WriteFile(socketPath, []byte{}, 0600); err != nil {
		t.Fatalf("failed to create stale socket: %v", err)
	}

	ln, err := ListenSocket(socketPath)
	if err != nil {
		t.Fatalf("ListenSocket() failed: %v", err)
	}
	defer ln.Close()

	// Verify we can connect to it
	if ln.Addr().String() != socketPath {
		t.Errorf("listener address is %q, want %q", ln.Addr().String(), socketPath)
	}
}

// TestListenSocket_RejectsInvalidDir verifies that ListenSocket rejects
// invalid directories and does NOT remove stale socket files from them.
// Per ADR-002, this validates the critical ordering invariant: directory
// validation must happen before any file removal to prevent TOCTOU attacks.
func TestListenSocket_RejectsInvalidDir(t *testing.T) {
	dir := t.TempDir()
	// Make directory world-readable (insecure)
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}

	socketPath := filepath.Join(dir, "test.sock")

	// Create a stale socket file
	if err := os.WriteFile(socketPath, []byte{}, 0600); err != nil {
		t.Fatalf("failed to create stale socket: %v", err)
	}

	_, err := ListenSocket(socketPath)
	if err == nil {
		t.Fatal("ListenSocket() should reject invalid directory")
	}
	t.Logf("correctly rejected: %v", err)

	// CRITICAL: Verify the stale socket was NOT removed.
	// Per ADR-002, we must never remove files from insecure directories.
	if _, statErr := os.Stat(socketPath); os.IsNotExist(statErr) {
		t.Fatal("ListenSocket() removed stale socket from insecure directory (security violation)")
	}
}
