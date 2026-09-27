package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"strings"

	"deploy-app/internal/buildkit"
	"deploy-app/internal/shellinfo"
)

// 运行壳相关资产。go:embed 与包路径绑定，因此集中放在根包，
// 通过 buildkit.Options 注入管线，保持管线与嵌入细节解耦。

//go:embed templates/base/base.exe
var baseExe []byte

//go:embed templates/base/base_version.txt
var baseShellManifest []byte

//go:embed templates/generated-app
var shellTemplates embed.FS

//go:embed internal/nginxproxy/path.go internal/resourcefs/resourcefs.go
var shellInternalSources embed.FS

// buildOptions 汇总打包管线所需的运行壳资产与一致性校验数据。
func buildOptions() (buildkit.Options, error) {
	hashes, err := computeShellHashes()
	if err != nil {
		return buildkit.Options{}, err
	}
	return buildkit.Options{
		BaseExe:       baseExe,
		ShellManifest: baseShellManifest,
		ShellHashes:   hashes,
	}, nil
}

// computeShellHashes 计算当前参与运行壳编译的全部源文件哈希。
// 清单以 internal/shellinfo.SourceFiles 为唯一来源，与 cmd/build-base 共用。
// 哈希前归一化 CRLF 为 LF，保证不同平台 checkout 出的文件算出相同哈希。
func computeShellHashes() (map[string]string, error) {
	hashes := make(map[string]string, len(shellinfo.SourceFiles))
	for _, name := range shellinfo.SourceFiles {
		var data []byte
		var err error
		if strings.HasPrefix(name, "templates/generated-app/") {
			data, err = shellTemplates.ReadFile(name)
		} else {
			data, err = shellInternalSources.ReadFile(name)
		}
		if err != nil {
			return nil, err
		}
		data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
		sum := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(sum[:])
	}
	return hashes, nil
}
