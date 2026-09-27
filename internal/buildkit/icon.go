package buildkit

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"deploy-app/internal/appconf"

	"github.com/tc-hib/winres"
	"github.com/tc-hib/winres/version"
)

// patchIcon 修补生成 exe 的图标与版本信息（PE 资源段）。
// 必须在追加尾部资源之前执行。
func patchIcon(exePath string, config appconf.BuildConfig) error {
	// Always refresh version metadata; icon is optional.
	exeData, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("读取 exe 失败: %w", err)
	}

	rs, err := winres.LoadFromEXE(bytes.NewReader(exeData))
	if err != nil {
		rs = &winres.ResourceSet{}
	}

	if config.IconPath != "" {
		icon, err := loadIcon(config.IconPath)
		if err != nil {
			return err
		}
		if err := replaceIconResource(rs, winres.ID(1), icon); err != nil {
			return fmt.Errorf("设置图标资源失败: %w", err)
		}
		if err := replaceIconResource(rs, winres.ID(3), icon); err != nil {
			return fmt.Errorf("设置窗口图标资源失败: %w", err)
		}
	}

	rs.SetManifest(winres.AppManifest{
		DPIAwareness:        winres.DPIPerMonitorV2,
		UseCommonControlsV6: true,
	})

	fileVer := parseVersion(config.Version)
	vi := version.Info{
		FileVersion:    fileVer,
		ProductVersion: fileVer,
	}
	product := config.AppName
	desc := strings.TrimSpace(config.Description)
	if desc == "" {
		desc = config.AppName
	}
	company := strings.TrimSpace(config.Company)
	setVI := func(key, value string) {
		if value == "" {
			return
		}
		// Prefer en-US for Windows Explorer Details; fall back to neutral.
		if err := vi.Set(version.LangDefault, key, value); err != nil {
			_ = vi.Set(version.LangNeutral, key, value)
		}
	}
	setVI(version.ProductName, product)
	setVI(version.FileDescription, desc)
	setVI(version.OriginalFilename, config.AppName+".exe")
	// CompanyName 是“公司”，LegalCopyright 对应资源管理器“详细信息 → 版权”。
	// Windows 11 属性页默认展示版权，不一定展示公司名，因此两者都写入。
	setVI(version.CompanyName, company)
	if company != "" {
		setVI(version.LegalCopyright, "Copyright © "+strconv.Itoa(time.Now().Year())+" "+company)
	}
	if v := strings.TrimSpace(config.Version); v != "" {
		setVI(version.ProductVersion, v)
		setVI(version.FileVersion, v)
	}
	rs.SetVersionInfo(vi)

	tmpPath := exePath + ".tmp"
	outFile, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	if err := rs.WriteToEXE(outFile, bytes.NewReader(exeData)); err != nil {
		outFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("写入 PE 资源失败: %w", err)
	}
	outFile.Close()

	if err := os.Remove(exePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("删除原文件失败: %w", err)
	}
	if err := os.Rename(tmpPath, exePath); err != nil {
		return fmt.Errorf("替换 exe 失败: %w", err)
	}
	return nil
}

func loadIcon(path string) (*winres.Icon, error) {
	ext := strings.ToLower(filepath.Ext(path))
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("读取图标失败: %w", err)
	}
	defer f.Close()

	switch ext {
	case ".png":
		img, _, err := image.Decode(f)
		if err != nil {
			return nil, fmt.Errorf("解码 PNG 图标失败: %w", err)
		}
		icon, err := winres.NewIconFromResizedImage(img, nil)
		if err != nil {
			return nil, fmt.Errorf("创建图标失败: %w", err)
		}
		return icon, nil
	case ".ico":
		icon, err := winres.LoadICO(f)
		if err != nil {
			return nil, fmt.Errorf("加载 ICO 图标失败: %w", err)
		}
		return icon, nil
	default:
		return nil, fmt.Errorf("不支持的图标格式: %s", ext)
	}
}

func parseVersion(v string) [4]uint16 {
	out := [4]uint16{1, 0, 0, 0}
	v = strings.TrimSpace(v)
	if v == "" {
		return out
	}
	parts := strings.Split(v, ".")
	for i := 0; i < len(parts) && i < 4; i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil || n < 0 || n > 65535 {
			return [4]uint16{1, 0, 0, 0}
		}
		out[i] = uint16(n)
	}
	return out
}

func replaceIconResource(rs *winres.ResourceSet, resID winres.ID, icon *winres.Icon) error {
	langIDs := []uint16{winres.LCIDNeutral, winres.LCIDDefault}
	oldLangIDs := make([]uint16, 0)
	addLangID := func(langID uint16) {
		for _, existing := range langIDs {
			if existing == langID {
				return
			}
		}
		langIDs = append(langIDs, langID)
	}
	rs.WalkType(winres.RT_GROUP_ICON, func(existingID winres.Identifier, langID uint16, _ []byte) bool {
		id, ok := existingID.(winres.ID)
		if !ok || id != resID {
			return true
		}
		oldLangIDs = append(oldLangIDs, langID)
		addLangID(langID)
		return true
	})
	for _, langID := range oldLangIDs {
		if err := rs.Set(winres.RT_GROUP_ICON, resID, langID, nil); err != nil {
			return err
		}
	}
	for _, langID := range langIDs {
		if err := rs.SetIconTranslation(resID, langID, icon); err != nil {
			return err
		}
	}
	return nil
}
