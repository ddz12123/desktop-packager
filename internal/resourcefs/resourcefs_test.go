package resourcefs

import (
	"archive/zip"
	"bytes"
	"errors"
	"io/fs"
	"testing"
)

func testZipFS(t *testing.T) fs.FS {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"index.html":       "<html>index</html>",
		"about.html":       "<html>about</html>",
		"assets/app.js":    "console.log(1)",
		"assets/img/x.png": "png",
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]*zip.File{}
	for _, f := range zr.File {
		entries[f.Name] = f
	}
	return NewZipFS(entries)
}

func TestZipFS_ReadFile(t *testing.T) {
	fsys := testZipFS(t)
	data, err := fs.ReadFile(fsys, "assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "console.log(1)" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestZipFS_ReadDir(t *testing.T) {
	fsys := testZipFS(t)
	entries, err := fs.ReadDir(fsys, "assets")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries under assets, got %d", len(entries))
	}
	byName := map[string]fs.DirEntry{}
	for _, e := range entries {
		byName[e.Name()] = e
	}
	if e := byName["app.js"]; e == nil || e.IsDir() {
		t.Fatalf("expected app.js file entry, got %+v", e)
	}
	if e := byName["img"]; e == nil || !e.IsDir() {
		t.Fatalf("expected img dir entry, got %+v", e)
	}

	rootEntries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(rootEntries) != 3 {
		t.Fatalf("expected 3 entries at root, got %d", len(rootEntries))
	}
}

func TestZipFS_NotExist(t *testing.T) {
	fsys := testZipFS(t)
	if _, err := fs.ReadFile(fsys, "nope.js"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestSPA_Fallback(t *testing.T) {
	fsys := NewSPA(testZipFS(t))

	// 无扩展名路由回退 index.html
	data, err := fs.ReadFile(fsys, "users/42")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<html>index</html>" {
		t.Fatalf("expected index.html fallback, got %q", string(data))
	}

	// 缺失的 .html 同样回退
	if _, err := fs.ReadFile(fsys, "missing-page.html"); err != nil {
		t.Fatalf("expected .html fallback, got %v", err)
	}

	// 静态资源扩展名保持真实 404
	if _, err := fs.ReadFile(fsys, "missing.js"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected static asset 404, got %v", err)
	}
	if _, err := fs.ReadFile(fsys, "missing.png"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected static asset 404, got %v", err)
	}

	// 已存在的文件不受回退影响
	data, err = fs.ReadFile(fsys, "about.html")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<html>about</html>" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}
