//go:build darwin || windows || linux

package ai

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractONNXRuntimeFromZip(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "runtime.zip")
	destPath := filepath.Join(tempDir, "onnxruntime.dll")
	const contents = "windows-runtime"

	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	entry, err := writer.Create("onnxruntime-win-x64-1.18.1/lib/onnxruntime.dll")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(contents)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractONNXRuntimeFromZip(archivePath, destPath, "onnxruntime.dll"); err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, destPath, contents)
}

func TestExtractONNXRuntimeFromTarGz(t *testing.T) {
	tempDir := t.TempDir()
	archivePath := filepath.Join(tempDir, "runtime.tgz")
	destPath := filepath.Join(tempDir, "libonnxruntime.so")
	const contents = "linux-runtime"

	archive, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(archive)
	tarWriter := tar.NewWriter(gzipWriter)
	payload := []byte(contents)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: "onnxruntime-linux-x64-1.18.1/lib/libonnxruntime.so.1.18.1",
		Mode: 0755,
		Size: int64(len(payload)),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractONNXRuntimeFromTarGz(archivePath, destPath, "libonnxruntime.so"); err != nil {
		t.Fatal(err)
	}
	assertFileContents(t, destPath, contents)
}

func assertFileContents(t *testing.T, path, expected string) {
	t.Helper()
	actual, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, []byte(expected)) {
		t.Fatalf("unexpected contents: %q", actual)
	}
}
