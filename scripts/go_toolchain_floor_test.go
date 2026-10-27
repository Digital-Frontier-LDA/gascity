package scripts_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCheckGoToolchainFloor verifies check-go-toolchain-floor.sh against a
// stub `go` that reports a chosen build version, so no binary is compiled:
//   - PASSES at and above the go.mod toolchain floor (patch, minor, suffix)
//   - FAILS below it, on a devel/unparseable version, and with no directive
//
// Regression guard for gascity#15: the agent image COPYs a prebuilt gc, so a
// golang:1.26.10 Dockerfile pin did not decide the shipped binary's toolchain;
// go.mod (go 1.26.6, no toolchain directive) did.
func TestCheckGoToolchainFloor(t *testing.T) {
	script := filepath.Join(repoRoot(t), "scripts", "check-go-toolchain-floor.sh")
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("check-go-toolchain-floor.sh not found at %s: %v", script, err)
	}

	const withFloor = "module github.com/example/mod\n\ngo 1.26.6\n\ntoolchain go1.26.10\n"

	run := func(t *testing.T, gomod, built string) (string, int) {
		t.Helper()
		dir := t.TempDir()
		writeTestFile(t, filepath.Join(dir, "go.mod"), gomod)
		writeTestFile(t, filepath.Join(dir, "gc"), "")
		binDir := filepath.Join(dir, "bin")
		if err := os.Mkdir(binDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		writeExecutable(t, filepath.Join(binDir, "go"), "#!/bin/sh\necho \"$2: "+built+"\"\n")
		cmd := exec.Command("bash", script, filepath.Join(dir, "gc"), filepath.Join(dir, "go.mod"))
		cmd.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		out, err := cmd.CombinedOutput()
		if err != nil {
			ex := &exec.ExitError{}
			if errors.As(err, &ex) {
				return string(out), ex.ExitCode()
			}
			t.Fatalf("exec error: %v", err)
		}
		return string(out), 0
	}

	for _, built := range []string{"go1.26.10", "go1.26.11", "go1.27.0", "go1.27", "go1.26.10 X:boringcrypto"} {
		t.Run("passes_"+strings.ReplaceAll(built, " ", "_"), func(t *testing.T) {
			if out, code := run(t, withFloor, built); code != 0 {
				t.Fatalf("expected exit 0 for %s, got %d\n%s", built, code, out)
			}
		})
	}

	for _, built := range []string{"go1.26.6", "go1.26.9", "go1.25.12", "devel go1.27-abcdef"} {
		t.Run("fails_"+strings.ReplaceAll(built, " ", "_"), func(t *testing.T) {
			out, code := run(t, withFloor, built)
			if code != 1 {
				t.Fatalf("expected exit 1 for %s, got %d\n%s", built, code, out)
			}
			if !strings.Contains(out, "ERROR:") {
				t.Fatalf("expected an ERROR line for %s, got:\n%s", built, out)
			}
		})
	}

	t.Run("fails_without_toolchain_directive", func(t *testing.T) {
		out, code := run(t, "module github.com/example/mod\n\ngo 1.26.6\n", "go1.27.0")
		if code != 1 || !strings.Contains(out, "no 'toolchain goX.Y.Z' directive") {
			t.Fatalf("expected exit 1 naming the missing directive, got %d\n%s", code, out)
		}
	})

	t.Run("repo_go_mod_declares_a_floor", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(repoRoot(t), "go.mod"))
		if err != nil {
			t.Fatalf("read go.mod: %v", err)
		}
		if !strings.Contains(string(data), "\ntoolchain go") {
			t.Fatalf("go.mod must keep a toolchain directive: setup-go installs it and check-go-toolchain-floor.sh checks shipped gc against it")
		}
	})
}
