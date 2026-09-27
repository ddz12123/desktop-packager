package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractZip_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"../evil.txt": "nope",
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zipPath, dest); err == nil {
		t.Fatal("expected zip slip error")
	}
}

func TestExtractZip_OK(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "ok.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"index.html":  "<html></html>",
		"assets/a.js": "console.log(1)",
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "index.html")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "assets", "a.js")); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZip_FileCountLimit(t *testing.T) {
	oldLimit := maxZipFiles
	maxZipFiles = 1
	defer func() { maxZipFiles = oldLimit }()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "many.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"a.txt": "a",
		"b.txt": "b",
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zipPath, dest); err == nil {
		t.Fatal("expected file count limit error")
	}
}

func TestExtractZip_TotalSizeLimit(t *testing.T) {
	oldLimit := maxZipTotalSize
	maxZipTotalSize = 4
	defer func() { maxZipTotalSize = oldLimit }()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "big.zip")
	if err := writeTestZip(zipPath, map[string]string{
		"index.html": "<html>too big</html>",
	}); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "out")
	if err := os.MkdirAll(dest, 0755); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zipPath, dest); err == nil {
		t.Fatal("expected total size limit error")
	}
}

func writeTestZip(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			zw.Close()
			return err
		}
		if _, err := w.Write([]byte(body)); err != nil {
			zw.Close()
			return err
		}
	}
	return zw.Close()
}
