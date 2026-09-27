// Package resourcefs 提供生成应用运行壳使用的只读文件系统适配器：
// 一个按需解压的 zip 条目 FS，以及一个 SPA 路由回退包装。
// 该文件由 cmd/build-base 原样复制进生成壳（仅替换 package 声明），
// 因此只允许依赖标准库。
package resourcefs

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

// NewZipFS 把 zip 条目（以资源 zip 内的路径为键，如 "assets/app.js"）包装成
// 只读 fs.FS。文件内容保留在 zip 中，读取时才解压。
func NewZipFS(files map[string]*zip.File) fs.FS {
	return &zipFS{files: files}
}

type zipFS struct {
	files map[string]*zip.File
}

func (z *zipFS) Open(name string) (fs.File, error) {
	name = path.Clean(name)
	if name == "." || name == "/" {
		return z.openDir(".")
	}
	name = strings.TrimPrefix(name, "/")

	if file, ok := z.files[name]; ok {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		return &zipOpenFile{
			name: path.Base(name),
			size: int64(file.UncompressedSize64),
			rc:   rc,
		}, nil
	}

	prefix := name + "/"
	for p := range z.files {
		if strings.HasPrefix(p, prefix) {
			return z.openDir(name)
		}
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

func (z *zipFS) openDir(dirPath string) (fs.File, error) {
	return &zipDir{name: path.Base(dirPath), path: dirPath, fs: z}, nil
}

type zipOpenFile struct {
	name string
	size int64
	rc   io.ReadCloser
}

func (f *zipOpenFile) Read(b []byte) (int, error) { return f.rc.Read(b) }
func (f *zipOpenFile) Close() error               { return f.rc.Close() }
func (f *zipOpenFile) Stat() (fs.FileInfo, error) {
	return &staticFileInfo{name: f.name, size: f.size, dir: false}, nil
}

type zipDir struct {
	name   string
	path   string
	fs     *zipFS
	offset int
	list   []fs.DirEntry
	init   bool
}

func (d *zipDir) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}
func (d *zipDir) Close() error { return nil }
func (d *zipDir) Stat() (fs.FileInfo, error) {
	return &staticFileInfo{name: d.name, dir: true}, nil
}

func (d *zipDir) ReadDir(n int) ([]fs.DirEntry, error) {
	if !d.init {
		d.list = d.readEntries()
		d.init = true
	}
	if n <= 0 {
		if d.offset >= len(d.list) {
			return nil, nil
		}
		out := d.list[d.offset:]
		d.offset = len(d.list)
		return out, nil
	}
	if d.offset >= len(d.list) {
		return nil, io.EOF
	}
	end := d.offset + n
	if end > len(d.list) {
		end = len(d.list)
	}
	out := d.list[d.offset:end]
	d.offset = end
	return out, nil
}

func (d *zipDir) readEntries() []fs.DirEntry {
	seen := map[string]bool{}
	var entries []fs.DirEntry
	dirPrefix := d.path
	if dirPrefix == "." {
		dirPrefix = ""
	} else {
		dirPrefix = dirPrefix + "/"
	}
	for filePath, zf := range d.fs.files {
		if !strings.HasPrefix(filePath, dirPrefix) {
			continue
		}
		rel := filePath[len(dirPrefix):]
		if rel == "" {
			continue
		}
		first := rel
		isDir := false
		if idx := strings.IndexByte(rel, '/'); idx >= 0 {
			first = rel[:idx]
			isDir = true
		}
		if seen[first] {
			continue
		}
		seen[first] = true
		if isDir {
			entries = append(entries, &staticDirEntry{name: first, dir: true})
		} else {
			entries = append(entries, &staticDirEntry{
				name: first,
				dir:  false,
				size: int64(zf.UncompressedSize64),
			})
		}
	}
	return entries
}

type staticFileInfo struct {
	name string
	size int64
	dir  bool
}

func (fi *staticFileInfo) Name() string { return fi.name }
func (fi *staticFileInfo) Size() int64  { return fi.size }
func (fi *staticFileInfo) Mode() fs.FileMode {
	if fi.dir {
		return fs.ModeDir | 0555
	}
	return 0444
}
func (fi *staticFileInfo) ModTime() time.Time { return time.Time{} }
func (fi *staticFileInfo) IsDir() bool        { return fi.dir }
func (fi *staticFileInfo) Sys() interface{}   { return nil }

type staticDirEntry struct {
	name string
	dir  bool
	size int64
}

func (e *staticDirEntry) Name() string { return e.name }
func (e *staticDirEntry) IsDir() bool  { return e.dir }
func (e *staticDirEntry) Type() fs.FileMode {
	if e.dir {
		return fs.ModeDir
	}
	return 0
}
func (e *staticDirEntry) Info() (fs.FileInfo, error) {
	return &staticFileInfo{name: e.name, size: e.size, dir: e.dir}, nil
}

// NewSPA 包装 root：无扩展名路由或缺失的 .html 路径回退到 index.html，
// 真实静态资源（.js/.css/图片字体等）仍返回 404。
func NewSPA(root fs.FS) fs.FS {
	return &spaFS{root: root}
}

type spaFS struct {
	root fs.FS
}

func (s *spaFS) Open(name string) (fs.File, error) {
	name = path.Clean("/" + name)
	if name == "/" {
		name = "."
	} else {
		name = strings.TrimPrefix(name, "/")
	}

	f, err := s.root.Open(name)
	if err == nil {
		return f, nil
	}
	if !isNotExist(err) {
		return nil, err
	}
	if shouldSPAFallback(name) {
		return s.root.Open("index.html")
	}
	return nil, err
}

func isNotExist(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return errors.Is(pe.Err, fs.ErrNotExist)
	}
	return false
}

func shouldSPAFallback(name string) bool {
	if name == "" || name == "." || name == "index.html" {
		return false
	}
	base := path.Base(name)
	if i := strings.LastIndex(base, "."); i > 0 {
		ext := strings.ToLower(base[i:])
		switch ext {
		case ".html", ".htm":
			return true
		case ".js", ".css", ".map", ".json", ".png", ".jpg", ".jpeg", ".gif", ".webp",
			".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot", ".mp3", ".mp4", ".webm",
			".txt", ".xml", ".wasm", ".mjs", ".cjs", ".pdf", ".zip":
			return false
		}
	}
	return true
}
