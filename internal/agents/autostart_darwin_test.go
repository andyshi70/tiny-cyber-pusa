//go:build darwin

package agents

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The launchd plist is the macOS equivalent of the HKCU Run entry, so it must
// come and go cleanly and never leave a loaded job behind.
func TestAutostartRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZEN_GATE_HOME", filepath.Join(home, "data"))

	if AutostartEnabled() {
		t.Fatal("autostart reported enabled on a fresh home")
	}
	if err := AutostartSet(true); err != nil {
		t.Fatalf("AutostartSet(true): %v", err)
	}
	t.Cleanup(func() { _ = AutostartSet(false) })

	plist := filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
	body, err := os.ReadFile(plist)
	if err != nil {
		t.Fatalf("plist not written: %v", err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	exe, _ = filepath.Abs(exe)
	for _, want := range []string{
		"<key>Label</key>",
		"<string>" + launchdLabel + "</string>",
		"<key>RunAtLoad</key>",
		"<key>ProgramArguments</key>",
		"<string>" + exe + "</string>",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("plist missing %q:\n%s", want, body)
		}
	}
	if !AutostartEnabled() {
		t.Fatal("AutostartEnabled false after enabling")
	}

	// bootstrap must have put the job in the user's gui domain. A headless
	// runner has no usable gui domain — `launchctl bootstrap` may even answer
	// success while the session never picks the job up (exit 113 on print) —
	// so the launchd half of the contract is only asserted off CI; the plist
	// assertions above still ran everywhere.
	if hasGUIDomain() && os.Getenv("CI") == "" {
		if err := exec.Command("launchctl", "print", launchdServiceTarget()).Run(); err != nil {
			t.Errorf("launchd job not loaded: %v", err)
		}
	}

	if err := AutostartSet(false); err != nil {
		t.Fatalf("AutostartSet(false): %v", err)
	}
	if _, err := os.Stat(plist); !os.IsNotExist(err) {
		t.Errorf("plist survived disable: %v", err)
	}
	if AutostartEnabled() {
		t.Error("AutostartEnabled true after disabling")
	}
	if hasGUIDomain() && os.Getenv("CI") == "" {
		if err := exec.Command("launchctl", "print", launchdServiceTarget()).Run(); err == nil {
			t.Error("launchd job still loaded after disable")
		}
	}
}

// hasGUIDomain reports whether this session has a per-user GUI launchd domain
// to bootstrap into. Over ssh and on CI runners it does not, and bootstrapping
// there fails for reasons that have nothing to do with this code.
func hasGUIDomain() bool {
	return exec.Command("launchctl", "print", launchdDomain()).Run() == nil
}

// bootstrap takes a domain-target and bootout a service-target; mixing them up
// is a usage error, so pin both spellings.
func TestLaunchctlTargetSpellings(t *testing.T) {
	if got, want := launchdDomain(), "gui/"; !strings.HasPrefix(got, want) {
		t.Fatalf("launchdDomain = %s, want prefix %s", got, want)
	}
	if got, want := launchdServiceTarget(), launchdDomain()+"/"+launchdLabel; got != want {
		t.Fatalf("launchdServiceTarget = %s, want %s", got, want)
	}
}
