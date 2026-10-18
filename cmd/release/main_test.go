package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"debug/macho"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildReleaseArtifacts(t *testing.T) {
	t.Chdir("../..")
	out := filepath.Join(t.TempDir(), "dist")
	if err := build("0.1.0", out); err != nil {
		t.Fatal(err)
	}
	formula, err := os.ReadFile(filepath.Join(out, "unfriction.rb"))
	if err != nil {
		t.Fatal(err)
	}
	checksums, err := os.ReadFile(filepath.Join(out, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		arch string
		cpu  macho.Cpu
	}{{"arm64", macho.CpuArm64}, {"amd64", macho.CpuAmd64}} {
		name := "unfriction_0.1.0_darwin_" + target.arch + ".tar.gz"
		data, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(data))
		if !strings.Contains(string(checksums), hash+"  "+name+"\n") {
			t.Fatal("checksum does not match archive")
		}
		if !strings.Contains(string(formula), hash) || !strings.Contains(string(formula), "/v0.1.0/"+name) {
			t.Fatal("formula version, URL, or checksum mismatch")
		}
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		tr := tar.NewReader(gz)
		header, err := tr.Next()
		if err != nil {
			t.Fatal(err)
		}
		if header.Name != "unfriction" || header.Mode != 0755 {
			t.Fatalf("unexpected archive entry: %+v", header)
		}
		binary, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		mf, err := macho.NewFile(bytes.NewReader(binary))
		if err != nil {
			t.Fatal(err)
		}
		if mf.Cpu != target.cpu {
			t.Fatalf("wrong CPU: %s", mf.Cpu)
		}
		if _, err := tr.Next(); err != io.EOF {
			t.Fatal("archive contains files beyond executable")
		}
		if err := gz.Close(); err != nil {
			t.Fatal(err)
		}
	}
	// Reruns may repair the formula, but must not replace published bytes.
	second := filepath.Join(t.TempDir(), "dist")
	if err := build("0.1.0", second); err != nil {
		t.Fatal(err)
	}
	other, err := os.ReadFile(filepath.Join(second, "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(checksums, other) {
		t.Fatal("release archives are not reproducible")
	}
}
