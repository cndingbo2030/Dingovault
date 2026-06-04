package bridge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunBlockCommandRejectsUnconfirmedUnsafeCommand(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker")
	app := NewApp(nil, nil, dir)

	_, err := app.RunBlockCommand("missing-block", fmt.Sprintf("printf hacked > %s", marker), "", false)
	if err == nil || !strings.Contains(err.Error(), "requires confirmation") {
		t.Fatalf("error = %v, want confirmation requirement", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("marker stat error = %v, want file not created", statErr)
	}
}
