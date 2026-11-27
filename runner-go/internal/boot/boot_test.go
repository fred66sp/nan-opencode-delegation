// Tests for Start: session registration in the store and the phase section.
package boot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func testRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".run"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

// sessionRow reports whether a session has a row in the store.
func sessionRow(t *testing.T, root, session string) (phase int, exists bool) {
	t.Helper()
	bin, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("sqlite3 not in PATH")
	}
	// Test IDs are literals, safe to interpolate; the CLI does not bind ?.
	out, err := exec.Command(bin, filepath.Join(root, ".run", "orchestrator.db"),
		"SELECT phase FROM sessions WHERE session = '"+session+"'").Output()
	if err != nil {
		return 0, false
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return 0, false
	}
	var value int
	if _, err := fmt.Sscanf(line, "%d", &value); err != nil {
		return 0, false
	}
	return value, true
}

func TestStartRegistersTheSessionInPhaseOne(t *testing.T) {
	root := testRoot(t)
	Start(root, "ses_test")
	phase, ok := sessionRow(t, root, "ses_test")
	if !ok {
		t.Fatal("runner start did not register the session")
	}
	if phase != 1 {
		t.Fatalf("session registered at phase %d, want 1", phase)
	}
}

func TestStartIsIdempotent(t *testing.T) {
	root := testRoot(t)
	Start(root, "ses_test")
	Start(root, "ses_test")
	phase, ok := sessionRow(t, root, "ses_test")
	if !ok {
		t.Fatal("second start removed the session row")
	}
	if phase != 1 {
		t.Fatalf("second start changed the phase to %d, want 1", phase)
	}
}
