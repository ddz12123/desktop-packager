package buildkit

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"deploy-app/internal/appconf"
)

const resourceFooterMagic uint32 = 0x5245534F // "RESO"

type resourceFooter struct {
	Magic  uint32
	Offset uint32
}

// appendResources 把资源 zip（dist/ + 两份配置）与 footer 流式追加到 exe 尾部，
// 不把整个 exe 载入内存。
func appendResources(ctx context.Context, exePath string, config appconf.BuildConfig, progress ProgressFunc) error {
	zipFile, err := os.CreateTemp(filepath.Dir(exePath), "resources-*.zip")
	if err != nil {
		return fmt.Errorf("创建临时 zip 失败: %w", err)
	}
	zipPath := zipFile.Name()
	defer func() {
		zipFile.Close()
		os.Remove(zipPath)
	}()

	zw := zip.NewWriter(zipFile)

	progress("写入代理配置", 60)
	proxyJSON, err := appconf.BuildProxyConfigJSON(config.ProxyRules)
	if err != nil {
		return err
	}
	if err := writeZipBytes(zw, "proxy_config.json", proxyJSON); err != nil {
		return err
	}

	appJSON, err := appconf.BuildAppConfigJSON(config)
	if err != nil {
		return err
	}
	if err := writeZipBytes(zw, "app_config.json", appJSON); err != nil {
		return err
	}

	progress("复制前端文件", 70)
	if err := addDirToZip(ctx, zw, config.DistPath, "dist"); err != nil {
		return fmt.Errorf("打包前端文件失败: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("关闭 zip 失败: %w", err)
	}
	if err := zipFile.Sync(); err != nil {
		return err
	}
	if _, err := zipFile.Seek(0, io.SeekStart); err != nil {
		return err
	}

	exeStat, err := os.Stat(exePath)
	if err != nil {
		return fmt.Errorf("读取 exe 信息失败: %w", err)
	}
	zipOffset := exeStat.Size()
	if zipOffset > math.MaxUint32 {
		return fmt.Errorf("可执行文件过大，超出资源偏移上限")
	}

	out, err := os.OpenFile(exePath, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return fmt.Errorf("打开 exe 追加写入失败: %w", err)
	}
	defer out.Close()

	progress("写入资源数据", 80)
	if _, err := io.Copy(out, &ctxReader{ctx: ctx, r: zipFile}); err != nil {
		return fmt.Errorf("写入 zip 数据失败: %w", err)
	}

	footer := resourceFooter{
		Magic:  resourceFooterMagic,
		Offset: uint32(zipOffset),
	}
	if err := binary.Write(out, binary.LittleEndian, &footer); err != nil {
		return fmt.Errorf("写入 footer 失败: %w", err)
	}
	return nil
}

func writeZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return fmt.Errorf("创建 zip 条目失败: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("写入 %s 失败: %w", name, err)
	}
	return nil
}

func addDirToZip(ctx context.Context, zw *zip.Writer, srcDir, zipPrefix string) error {
	return filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		zipName := zipPrefix + "/" + filepath.ToSlash(relPath)
		if d.IsDir() {
			_, err := zw.Create(zipName + "/")
			return err
		}
		w, err := zw.Create(zipName)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		f.Close()
		return copyErr
	})
}
