package cmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMakefile_LinuxDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX host")
	}
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make command not available in environment")
	}

	targets := []string{"build", "clean", "help", "windows", "linux"}
	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "make", "-n", target)
			cmd.Dir = ".."
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("make -n %s failed: %v, output: %s", target, err, string(out))
			}
			outStr := string(out)

			switch target {
			case "build":
				if !strings.Contains(outStr, "bin/xops") {
					t.Fatalf("expected bin/xops in build output, got: %s", outStr)
				}
				if strings.Contains(outStr, "CGO_ENABLED=0 go build") {
					t.Fatalf("unexpected inline CGO_ENABLED=0 shell syntax in build output: %s", outStr)
				}
			case "clean":
				if !strings.Contains(outStr, "rm -rf bin coverage.out") {
					t.Fatalf("expected rm -rf in clean output, got: %s", outStr)
				}
			case "windows":
				if !strings.Contains(outStr, "bin/xops.exe") {
					t.Fatalf("expected bin/xops.exe in windows output, got: %s", outStr)
				}
			}
		})
	}
}

func TestMakefile_WindowsDryRun(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make command not available in environment")
	}

	t.Run("Windows_NT_build", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "make", "-n", "OS=Windows_NT", "build")
		cmd.Dir = ".."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("make -n OS=Windows_NT build failed: %v, output: %s", err, string(out))
		}
		outStr := string(out)
		if !strings.Contains(outStr, "bin/xops.exe") {
			t.Fatalf("expected bin/xops.exe with OS=Windows_NT, got: %s", outStr)
		}
		if strings.Contains(outStr, "CGO_ENABLED=0 go build") {
			t.Fatalf("unexpected inline CGO_ENABLED=0 shell syntax in build output: %s", outStr)
		}
	})

	t.Run("Windows_NT_POSIX_clean", func(t *testing.T) {
		// This simulates the Windows POSIX branch on a POSIX host.
		// Native Windows Make can fall back to cmd even with SHELL=/bin/bash.
		if runtime.GOOS == "windows" {
			t.Skip("requires a POSIX host; native Windows clean is tested separately")
		}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "make", "-n", "OS=Windows_NT", "SHELL=/bin/bash", "clean")
		cmd.Dir = ".."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("make -n OS=Windows_NT SHELL=/bin/bash clean failed: %v, output: %s", err, string(out))
		}
		outStr := string(out)
		if !strings.Contains(outStr, "rm -rf bin coverage.out") {
			t.Fatalf("expected rm -rf for POSIX shell on Windows_NT, got: %s", outStr)
		}
	})

	t.Run("Windows_NT_CMD_clean", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		// Avoid metadata subprocesses so the CMD dry run also works on POSIX hosts.
		cmd := exec.CommandContext(ctx, "make", "-n", "OS=Windows_NT", "SHELL=cmd.exe",
			"GOPATH_BIN=", "VERSION=test", "COMMIT=test", "DATE=test", "clean")
		cmd.Dir = ".."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("native Windows clean dry run failed: %v, output: %s", err, out)
		}
		outStr := string(out)
		if !strings.Contains(outStr, "rmdir /s /q bin") {
			t.Fatalf("expected rmdir /s /q bin for CMD shell on Windows_NT, got: %s", outStr)
		}
		if !strings.Contains(outStr, "del /f /q coverage.out") {
			t.Fatalf("expected del /f /q coverage.out for CMD shell on Windows_NT, got: %s", outStr)
		}
	})
}

func TestMakefile_CrossCompilationTargets(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make command not available in environment")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "make", "-n", "windows", "windows-arm64", "linux", "linux-arm64", "darwin-amd64", "darwin-arm64")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make -n cross compilation targets failed: %v, output: %s", err, string(out))
	}

	outStr := string(out)
	expectedArtifacts := []string{
		"bin/xops.exe",
		"bin/xops-arm64.exe",
		"bin/xops-linux-amd64",
		"bin/xops-linux-aarch64",
		"bin/xops-darwin-amd64",
		"bin/xops-darwin-arm64",
	}

	for _, artifact := range expectedArtifacts {
		if !strings.Contains(outStr, artifact) {
			t.Errorf("expected cross compilation dry-run to contain %s, got output: %s", artifact, outStr)
		}
	}
}

func TestMakefile_TimeoutContextSafety(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make command not available in environment")
	}
	// Bound make execution even when the suite has no timeout.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	start := time.Now()
	cmd := exec.CommandContext(ctx, "make", "-n", "help")
	cmd.Dir = ".."
	if err := cmd.Run(); err != nil {
		t.Fatalf("make -n help failed: %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("make -n help took unexpectedly long: %v", time.Since(start))
	}
}

// Exercise both POSIX branches with a real copy into a home path containing spaces.
func TestMakefile_InstallSkillPOSIX(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX host")
	}
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make command not available in environment")
	}
	makefile, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	for _, platform := range []string{"Linux", "Windows_NT"} {
		t.Run(platform, func(t *testing.T) {
			root := t.TempDir()
			skillDir := filepath.Join(root, "skills", "xops-agent")
			if err := os.MkdirAll(skillDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "Makefile"), makefile, 0o600); err != nil {
				t.Fatal(err)
			}
			const content = "skill installation fixture"
			if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			homeDir := filepath.Join(root, "home with spaces")
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "make", "OS="+platform, "SHELL=/bin/sh", "GOPATH_BIN=", "HOME="+homeDir, "install-skill")
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("install-skill failed: %v, output: %s", err, out)
			}
			got, err := os.ReadFile(filepath.Join(homeDir, ".gemini", "skills", "xops-agent", "SKILL.md"))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != content {
				t.Fatalf("installed content = %q, want %q", got, content)
			}
		})
	}
}

func TestMakefile_InstallSkillWindowsDryRun(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make command not available in environment")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	// Override shell-evaluated metadata so this dry run also works on POSIX hosts.
	cmd := exec.CommandContext(ctx, "make", "-n", "OS=Windows_NT", "SHELL=cmd.exe",
		"GOPATH_BIN=", "VERSION=test", "COMMIT=test", "DATE=test", "install-skill")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Windows install-skill dry run failed: %v, output: %s", err, out)
	}
	if strings.Count(string(out), "(Join-Path $HOME '.gemini/skills/xops-agent')") != 2 {
		t.Fatalf("expected expandable home paths for both installation commands, got: %s", out)
	}
}

// Run a tiny real Go build so Go's own -ldflags parser checks the quoting.
func TestMakefile_MetadataWithSpaces(t *testing.T) {
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make command not available in environment")
	}
	root := makefileFixture(t)
	writeMakefileFixture(t, root, "go.mod", "module example.test\n\ngo 1.26.0\n")
	writeMakefileFixture(t, root, "cmd/version/version.go", "package version\nvar Version, Commit, BuildTime string\n")
	writeMakefileFixture(t, root, "cmd/cli/main.go", `package main
import ("fmt"; "example.test/cmd/version")
func main() { fmt.Printf("%s|%s|%s", version.Version, version.Commit, version.BuildTime) }
`)
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "build", "MODULE=example.test",
		"VERSION=release candidate", "COMMIT=custom commit", "DATE=2026-09-14 12:00:00")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build with spaced metadata failed: %v, output: %s", err, out)
	}
	binary := "xops"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	out, err := exec.CommandContext(ctx, filepath.Join(root, "bin", binary)).CombinedOutput()
	if err != nil {
		t.Fatalf("run metadata fixture failed: %v, output: %s", err, out)
	}
	if want := "release candidate|custom commit|2026-09-14 12:00:00"; string(out) != want {
		t.Fatalf("metadata = %q, want %q", out, want)
	}
}

func TestMakefile_WindowsPOSIXPath(t *testing.T) {
	// Exercise recursive Make even when this test is invoked directly with go test.
	t.Setenv("MAKELEVEL", "1")
	if runtime.GOOS == "windows" {
		t.Skip("requires a POSIX host for shell fixtures")
	}
	makePath, err := exec.LookPath("make")
	if err != nil {
		t.Skip("make command not available in environment")
	}
	for _, convert := range []bool{false, true} {
		name := "POSIX_GOPATH"
		if convert {
			name = "Windows_GOPATH_with_cygpath"
		}
		t.Run(name, func(t *testing.T) {
			root := makefileFixture(t)
			writeMakefileFixture(t, root, "tools/go", "#!/bin/sh\nprintf '%s\\n' \"$FIXTURE_GOPATH\"\n")
			writeMakefileFixture(t, root, "go home/bin/path-probe", "#!/bin/sh\nprintf 'go-bin-ok'\n")
			writeMakefileFixture(t, root, "tools/original-probe", "#!/bin/sh\nprintf 'original-ok'\n")
			if convert {
				writeMakefileFixture(t, root, "tools/cygpath", "#!/bin/sh\n[ \"$1\" = -u ] && [ \"$2\" = 'C:\\Go Home/bin' ] || exit 1\nprintf '%s\\n' \"$FIXTURE_BIN\"\n")
			}
			// Both tools must resolve: one in Go bin and one in the original first PATH entry.
			writeMakefileFixture(t, root, "probe.mk", "include Makefile\nprobe:\n\t@path-probe\n\t@original-probe\n")
			t.Setenv("PATH", filepath.Join(root, "tools"))
			t.Setenv("FIXTURE_BIN", filepath.Join(root, "go home", "bin"))
			gopath := filepath.Join(root, "go home")
			if convert {
				gopath = `C:\Go Home`
			}
			t.Setenv("FIXTURE_GOPATH", gopath)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, makePath, "--no-print-directory", "-f", "probe.mk", "OS=Windows_NT", "SHELL=/bin/sh",
				"VERSION=test", "COMMIT=test", "DATE=test", "probe")
			cmd.Dir = root
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("PATH probe failed: %v, output: %s", err, out)
			}
			if string(out) != "go-bin-okoriginal-ok" {
				t.Fatalf("unexpected PATH probe output: %s", out)
			}
		})
	}
}

func makefileFixture(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile("../Makefile")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	writeMakefileFixture(t, root, "Makefile", string(content))
	return root
}

func writeMakefileFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

// Model a Windows Make that reports sh.exe but dispatches commands to cmd.exe.
func TestMakefile_WindowsShellFallback(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX script to emulate cmd quoting")
	}
	if _, err := exec.LookPath("make"); err != nil {
		t.Skip("make command not available in environment")
	}
	root := makefileFixture(t)
	writeMakefileFixture(t, root, "sh.exe", `#!/bin/sh
case "$2" in
  "echo 'xops-posix-shell' && echo 'xops-posix-shell'") printf "'xops-posix-shell'\n'xops-posix-shell'\n" ;;
  "go env GOPATH "*) printf "/nonexistent-fixture-gopath\n" ;;
  *) printf 'unexpected shell command: %s\n' "$2" >&2; exit 1 ;;
esac
`)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "-n", "OS=Windows_NT",
		"SHELL="+filepath.Join(root, "sh.exe"), "GOPATH_BIN=",
		"VERSION=test", "COMMIT=test", "DATE=test", "clean", "build", "install-skill")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fallback dry run failed: %v, output: %s", err, out)
	}
	output := string(out)
	if strings.Contains(output, "unexpected shell command") {
		t.Fatalf("executed a POSIX command despite cmd fallback: %s", output)
	}
	for _, want := range []string{"rmdir /s /q bin", "del /f /q coverage.out", "bin/xops.exe", "powershell -NoProfile"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected native Windows command %q, got: %s", want, output)
		}
	}
}
