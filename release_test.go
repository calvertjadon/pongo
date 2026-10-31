//go:build linux || darwin

package main_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// The release workflow sets these variables to the archive built for the runner's operating system and architecture.
const (
	releaseArchiveEnvironment   = "PONGO_RELEASE_ARCHIVE"
	releaseChecksumsEnvironment = "PONGO_RELEASE_SUMS"
	releaseVersionEnvironment   = "PONGO_RELEASE_VERSION"
)

// TestReleaseArchive checks one published release archive on the platform it targets: its checksum, its contents, and
// the executable's command and terminal behavior. The release workflow runs it on a runner matching each target. It is
// skipped elsewhere, because only that runner can execute the executable inside the archive.
func TestReleaseArchive(t *testing.T) {
	archive := os.Getenv(releaseArchiveEnvironment)
	if archive == "" {
		t.Skipf("set %s to a release archive to check it", releaseArchiveEnvironment)
	}
	t.Parallel()

	version := os.Getenv(releaseVersionEnvironment)
	if version == "" {
		t.Fatalf("%s is set without %s", releaseArchiveEnvironment, releaseVersionEnvironment)
	}
	expected := fmt.Sprintf("pongo_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
	if name := filepath.Base(archive); name != expected {
		t.Fatalf("archive is named %q, want %q for this platform", name, expected)
	}

	verifyChecksum(t, archive, os.Getenv(releaseChecksumsEnvironment))
	executable := extractRelease(t, archive)
	verifyReleaseInvocations(t, executable, version)
	verifyReleaseSessions(t, executable)
}

// verifyChecksum checks the archive against its record in the release's checksum file.
func verifyChecksum(t *testing.T, archive, checksums string) {
	t.Helper()

	if checksums == "" {
		t.Fatalf("%s is not set", releaseChecksumsEnvironment)
	}
	contents, err := os.ReadFile(checksums)
	if err != nil {
		t.Fatalf("read checksums: %v", err)
	}
	name := filepath.Base(archive)
	recorded := ""
	for _, line := range strings.Split(strings.TrimSpace(string(contents)), "\n") {
		if checksum, file, found := strings.Cut(line, "  "); found && file == name {
			recorded = checksum
		}
	}
	if recorded == "" {
		t.Fatalf("%s lists no checksum for %s", releaseChecksumsEnvironment, name)
	}
	if actual := checksumOf(t, archive); actual != recorded {
		t.Fatalf("%s has checksum %s, want %s", name, actual, recorded)
	}
}

// checksumOf returns the lowercase hexadecimal SHA-256 checksum of a file.
func checksumOf(t *testing.T, path string) string {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer closeOrReport(t, file)

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	return hex.EncodeToString(hash.Sum(nil))
}

// extractRelease unpacks the archive into a temporary directory after checking that it holds exactly the executable
// and the MIT License, then returns the extracted executable.
func extractRelease(t *testing.T, archive string) string {
	t.Helper()

	file, err := os.Open(archive)
	if err != nil {
		t.Fatalf("open release archive: %v", err)
	}
	defer closeOrReport(t, file)
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatalf("decompress release archive: %v", err)
	}
	defer closeOrReport(t, compressed)

	directory := t.TempDir()
	found := make(map[string]bool)
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("read release archive: %v", err)
		}
		if header.Typeflag != tar.TypeReg {
			t.Fatalf("release archive holds %q, which is not a regular file", header.Name)
		}
		if header.Name != "pongo" && header.Name != "LICENSE" {
			t.Fatalf("release archive holds unexpected %q", header.Name)
		}
		if header.Name == "pongo" && header.Mode&0o111 == 0 {
			t.Fatalf("release archive holds pongo without execute permission")
		}
		writeMember(t, filepath.Join(directory, header.Name), reader, os.FileMode(header.Mode).Perm())
		found[header.Name] = true
	}
	for _, wanted := range []string{"pongo", "LICENSE"} {
		if !found[wanted] {
			t.Fatalf("release archive does not hold %s", wanted)
		}
	}
	verifyLicense(t, filepath.Join(directory, "LICENSE"))

	return filepath.Join(directory, "pongo")
}

// writeMember writes one archive member to path.
func writeMember(t *testing.T, path string, reader io.Reader, mode os.FileMode) {
	t.Helper()

	contents, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read release archive member: %v", err)
	}
	if err := os.WriteFile(path, contents, mode); err != nil {
		t.Fatalf("write release archive member: %v", err)
	}
}

// verifyLicense checks that the archive carries the MIT License.
func verifyLicense(t *testing.T, path string) {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read license: %v", err)
	}
	text := string(contents)
	if !strings.Contains(text, "MIT License") || !strings.Contains(text, "Jadon Calvert") {
		t.Fatalf("release archive does not hold the MIT License")
	}
}

// verifyReleaseInvocations checks the documented command-line behavior of the released executable.
func verifyReleaseInvocations(t *testing.T, executable, version string) {
	t.Helper()

	if reported := reportedVersion(t, executable); reported != version {
		t.Fatalf("pongo --version printed %q, want %q", reported, version)
	}
	for _, argument := range []string{"--help", "-h"} {
		result := runWithoutTerminal(t, executable, argument)
		if result.status != 0 || result.stdout == "" || result.stderr != "" {
			t.Fatalf("pongo %s exited %d, stdout %q, stderr %q; want usage on standard output",
				argument, result.status, result.stdout, result.stderr)
		}
	}
	for _, arguments := range [][]string{{"--bogus"}, {"play"}, {"--help", "extra"}} {
		result := runWithoutTerminal(t, executable, arguments...)
		if result.status != 2 || result.stderr == "" || result.stdout != "" {
			t.Fatalf("pongo %q exited %d, stdout %q, stderr %q; want a usage error on standard error",
				arguments, result.status, result.stdout, result.stderr)
		}
	}
	t.Run("no terminal", func(t *testing.T) {
		t.Parallel()

		var stdout, stderr bytes.Buffer
		command := exec.Command(executable)
		command.Stdout = &stdout
		command.Stderr = &stderr
		// A new session has no controlling terminal, so nothing can fall back to /dev/tty.
		command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

		assertStartupRejected(t, command, &stderr)
	})
}

// verifyReleaseSessions checks pseudo-terminal startup, quitting, and catchable-signal cleanup of the released
// executable.
func verifyReleaseSessions(t *testing.T, executable string) {
	t.Helper()

	for _, exit := range sessionExits() {
		t.Run(exit.name, func(t *testing.T) {
			t.Parallel()

			assertSessionExit(t, executable, exit)
		})
	}
}

// closeOrReport closes a file read from a release archive and reports a close failure.
func closeOrReport(t *testing.T, closer io.Closer) {
	t.Helper()

	if err := closer.Close(); err != nil {
		t.Errorf("close release archive: %v", err)
	}
}
