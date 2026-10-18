// release is an internal CI utility. It never publishes or pushes by itself.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/unfriction-ai/unfriction/internal/release"
)

func main() {
	if err := execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute() error {
	mode := flag.String("mode", "plan", "plan or build")
	version := flag.String("version", "", "version to embed in artifacts")
	out := flag.String("out", "dist", "output directory")
	flag.Parse()
	switch *mode {
	case "plan":
		return plan()
	case "build":
		return build(*version, *out)
	default:
		return fmt.Errorf("unknown mode %q", *mode)
	}
}

func git(args ...string) (string, error) {
	b, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, b)
	}
	return strings.TrimSpace(string(b)), nil
}

func plan() error {
	tags, err := git("tag", "--merged", "HEAD", "--list", "v*", "--sort=-version:refname")
	if err != nil {
		return err
	}
	previous := ""
	for _, tag := range strings.Fields(tags) {
		if regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(tag) {
			previous = tag
			break
		}
	}
	rangeSpec := "HEAD"
	if previous != "" {
		head, err := git("rev-parse", "HEAD")
		if err != nil {
			return err
		}
		tagged, err := git("rev-list", "-n", "1", previous)
		if err != nil {
			return err
		}
		if head == tagged {
			// A rerun repairs publication or repeats the install smoke test;
			// it must never calculate a new version for the same commit.
			return json.NewEncoder(os.Stdout).Encode(release.Plan{Version: strings.TrimPrefix(previous, "v"), Tag: previous})
		}
		rangeSpec = previous + "..HEAD"
	}
	log, err := git("log", "--format=%B%x00", rangeSpec)
	if err != nil {
		return err
	}
	messages := strings.Split(log, "\x00")
	for i := range messages {
		messages[i] = strings.TrimSpace(messages[i])
	}
	p, err := release.Next(previous, messages)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(p)
}

func build(version, out string) error {
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(version) {
		return fmt.Errorf("expected a release version, got %q", version)
	}
	// Refuse to mix artifacts from different versions or retries.
	if err := os.Mkdir(out, 0755); err != nil {
		return err
	}
	hashes := map[string]string{}
	var checksums strings.Builder
	for _, arch := range []string{"arm64", "amd64"} {
		dir, err := os.MkdirTemp("", "unfriction-release-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dir)
		binary := filepath.Join(dir, "unfriction")
		cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-ldflags", "-s -w -X main.version="+version, "-o", binary, "./cmd/unfriction")
		cmd.Env = append(os.Environ(), "GOOS=darwin", "GOARCH="+arch, "CGO_ENABLED=0")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("build %s: %w: %s", arch, err, output)
		}
		data, err := os.ReadFile(binary)
		if err != nil {
			return err
		}
		var buffer bytes.Buffer
		gz := gzip.NewWriter(&buffer)
		tw := tar.NewWriter(gz)
		// Only the executable goes into the public archive; never source files.
		if err := tw.WriteHeader(&tar.Header{Name: "unfriction", Mode: 0755, Size: int64(len(data))}); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
		if err := tw.Close(); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}
		name := fmt.Sprintf("unfriction_%s_darwin_%s.tar.gz", version, arch)
		if err := os.WriteFile(filepath.Join(out, name), buffer.Bytes(), 0644); err != nil {
			return err
		}
		hashes[arch] = fmt.Sprintf("%x", sha256.Sum256(buffer.Bytes()))
		fmt.Fprintf(&checksums, "%s  %s\n", hashes[arch], name)
	}
	if err := os.WriteFile(filepath.Join(out, "checksums.txt"), []byte(checksums.String()), 0644); err != nil {
		return err
	}
	formula := fmt.Sprintf(`class Unfriction < Formula
  desc "Inspect recorded coding-agent runs"
  homepage "https://github.com/unfriction-ai/unfriction-releases"
  version "%s"
  depends_on :macos

  on_arm do
    url "https://github.com/unfriction-ai/unfriction-releases/releases/download/v%s/unfriction_%s_darwin_arm64.tar.gz"
    sha256 "%s"
  end
  on_intel do
    url "https://github.com/unfriction-ai/unfriction-releases/releases/download/v%s/unfriction_%s_darwin_amd64.tar.gz"
    sha256 "%s"
  end

  def install
    bin.install "unfriction"
  end

  test do
    assert_equal "unfriction #{version}\n", shell_output("#{bin}/unfriction --version")
  end
end
`, version, version, version, hashes["arm64"], version, version, hashes["amd64"])
	return os.WriteFile(filepath.Join(out, "unfriction.rb"), []byte(formula), 0644)
}
