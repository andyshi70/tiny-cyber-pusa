package agents

import (
	"os"
	"path/filepath"
	"testing"

	"zen-gate/internal/lane"
	"zen-gate/internal/store"
)

func TestRegistryEnableReportsHermesBackup(t *testing.T) {
	t.Setenv("ZEN_GATE_HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("HERMES_HOME", home)

	const cfg = "model:\n  default: keep-me\n"
	if err := os.WriteFile(filepath.Join(home, "config.yaml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open()
	if err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(st)
	r.SetEndpoints("http://127.0.0.1:8787/v1", []lane.ModelInfo{{ID: "mimo-v2.6-flash-free"}})

	changes, err := r.EnableWithReport("hermes")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("changes = %+v, want exactly one file", changes)
	}
	if changes[0].File != filepath.Join(home, "config.yaml") {
		t.Errorf("file = %q", changes[0].File)
	}
	if changes[0].Backup == "" {
		t.Fatal("no backup path reported")
	}
	if got := readFile(t, changes[0].Backup); got != cfg {
		t.Errorf("backup content = %q, want the pre-enable config", got)
	}

	changes, err = r.DisableWithReport("hermes")
	if err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 {
		t.Fatalf("disable changes = %+v, want exactly one file", changes)
	}
	if changes[0].Backup == "" {
		t.Fatal("disable reported no backup path")
	}
}
