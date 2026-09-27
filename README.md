# Deploy App

![Platform](https://img.shields.io/badge/platform-Windows-blue)
![Wails](https://img.shields.io/badge/Wails-v2-ff69b4)
![Vue](https://img.shields.io/badge/Vue-3-42b883)
![License](https://img.shields.io/badge/license-MIT-green)

Deploy App 是一个基于 Wails 的 Windows 桌面应用打包工具。它可以把已经构建好的前端项目（Vue、React、Angular、静态站点等）打包成独立的 `.exe` 应用，并支持自定义图标、窗口大小和反向代理规则。

这个项目的目标很直接：让前端应用在不改业务代码的情况下，快速生成一个可分发的 Windows 桌面程序。

> 👉 **新用户请先阅读《[使用说明](使用说明.md)》**：三分钟上手、各步骤详解、方案管理与常见问题。

## 界面预览

### 导入构建产物

![导入构建产物](assets/img/step-1.png)

### 应用配置

![应用配置](assets/img/step-2.png)

### 反向代理配置

![反向代理配置](assets/img/step-3.png)

### 构建生成

![构建生成](assets/img/step-4.png)

## 功能特性

- 单文件输出：生成一个独立的 `.exe` 文件，便于复制和分发。
- 零 Go 依赖打包：最终用户使用打包工具时，不需要安装 Go 环境。
- 自定义应用图标：支持 `.ico` 和 `.png` 图标，并同步到 exe 图标和窗口标题栏图标。
- 窗口配置：支持窗口宽度、高度、最大化、全屏，以及关闭前确认。
- 版本信息：支持写入版本号、描述、公司/组织到 PE 版本资源。
- 反向代理：内置与 nginx `location` + `proxy_pass` 对齐的路径语义，适合接口跨域或本地服务转发。
- SPA 路由回退：前端 History 路由刷新未知路径时回退到 `index.html`。
- 静态资源嵌入：前端 `dist` 文件会被打进 exe 内部，运行时自动加载。
- ZIP 导入：支持直接选择 `dist` 文件夹、上传构建产物 ZIP 包，或把文件夹/ZIP **直接拖入窗口**（带 Zip Slip 防护、解压上限与临时目录清理）。
- 导入即建议应用名：自动读取 dist 同级 `package.json` 的 name 字段预填应用名。
- 代理连通性测试：配置代理规则时可一键测试目标地址是否可达。
- 构建可控：构建前选择保存位置，构建过程中可随时取消。
- 运行壳一致性校验：`base.exe` 附带源文件哈希清单，模板变更后未重新生成会直接报错提示。
- 配置持久化与方案管理：自动记住上次配置；可把整套构建配置保存为命名方案，支持载入、删除、导出、导入，方便团队复用。
- 生成应用增强：单实例锁（重复启动唤起已有窗口）、记住窗口位置/大小、窗口标题独立于 exe 文件名。
- 外置配置覆盖：生成应用支持 exe 同目录 `app_config.json` / `proxy_config.json` 覆盖内置配置，改代理目标无需重新打包。
- 代码签名：构建后生成签名脚本（自动定位 signtool），证书密码运行时输入、不落盘。
- 试运行：构建完成后可一键启动生成的 exe 做冒烟测试。
- CLI 模式：`deploy-app build --config 方案.json` 无界面打包，可接入 CI 流水线。
- 检查更新与在线更新：设置面板一键检查 GitHub Releases 最新版本，支持自动下载替换重启。

## 适用场景

- 将内部管理后台快速封装为 Windows 桌面应用。
- 给纯前端项目生成可双击运行的交付包。
- 需要把前端静态资源和少量代理配置打包进一个 exe。
- 希望分发给非技术用户，不要求对方安装 Node.js、Go 或命令行工具。

## 快速使用

### 使用发布版本

1. 从 GitHub Releases 下载发行包，两种形式任选：
   - **绿色版** `deploy-app-vX.Y.Z.exe`：下载即用；
   - **安装版** `deploy-app-vX.Y.Z-setup.exe`：自动创建快捷方式、自带卸载器（用户级安装，无需管理员权限）。
2. 双击运行。
3. 导入前端项目的 `dist` 目录，或上传包含构建产物的 ZIP 包。
4. 设置应用名称、图标、窗口大小和代理规则。
5. 点击“开始构建”，选择保存位置，生成最终 exe。

### ZIP 包格式

支持两种常见结构：

```text
dist.zip
  index.html
  assets/
  ...
```

```text
dist.zip
  dist/
    index.html
    assets/
    ...
```

## 从源码运行

### 环境要求

- Windows 10/11 64-bit
- Go 1.24+
- Node.js 20+
- Wails CLI v2

### 安装依赖

```bash
cd frontend
npm install
cd ..
```

### 启动开发模式

```bash
wails dev
```

### 构建前端

```bash
cd frontend
npm run build
cd ..
```

### 重新生成基础运行壳（重要）

修改 `templates/generated-app/` 下的模板，或修改 `internal/nginxproxy/`、`internal/resourcefs/` 中与运行壳共享的实现后，必须重新生成基础 exe，否则打包工具会直接报错提示漂移：

```bash
go run ./cmd/build-base
```

输出位置：

```text
templates/base/base.exe
templates/base/base_version.txt   # 源文件 SHA256 清单，由工具生成，请勿手改
```

> 注意：仓库中的 `templates/base/base.exe` 需要是真实可运行的 Wails 壳。如果只有占位文件，请先执行上面的命令生成。

### 构建 Deploy App

```bash
wails build
```

构建产物默认输出到：

```text
build/bin/deploy-app.exe
```

## 使用流程

### 1. 导入前端构建产物

选择前端项目构建后的 `dist` 文件夹，或上传 ZIP 压缩包。目录中必须包含 `index.html`。

可在全局设置中指定临时目录；未设置时，ZIP 解压和构建临时文件会落在导入目录附近，并在关闭应用时清理。

### 2. 配置应用信息

| 字段 | 说明 |
| --- | --- |
| 应用名称 | 作为 exe 文件名，禁止 Windows 非法字符与保留名（如 `CON`） |
| 版本号 | 如 `1.0.0`，写入 PE 版本信息 |
| 描述 | 可选，写入文件说明（FileDescription） |
| 公司/组织 | 可选，写入公司名，并生成详细信息中的版权（LegalCopyright） |
| 图标 | `.ico` 或正方形 `.png`（建议 ≥256） |
| 窗口标题 | 可选，留空则使用应用名称 |
| 窗口 | 宽高 / 最大化 / 全屏 / 关闭前确认 / 单实例锁 / 记住窗口位置 |
| 代码签名 | 可选 PFX 证书与 RFC3161 时间戳服务器，构建后在输出 exe 旁生成同名 `-sign.cmd` 签名脚本 |

### 3. 配置反向代理（与 nginx 对齐）

代理规则对应 nginx 的：

```nginx
location <路径前缀> {
    proxy_pass <目标地址>;
}
```

“重写为”非空时，等价于覆盖 `proxy_pass` 的 URI 部分。

#### 路径处理规则

| 配置方式 | nginx 对应 | 行为 | 示例 |
| --- | --- | --- | --- |
| 目标地址**无路径** | `proxy_pass http://host:8080;` | 保留完整请求路径 | `/api/users` → `/api/users` |
| 目标地址以 `/` 结尾 | `proxy_pass http://host:8080/;` | 剥离 location 前缀后拼接 | `/api/users` → `/users` |
| 目标地址带前缀路径 | `proxy_pass http://host:8080/v2/;` | 用该路径替换 location 前缀 | `/api/users` → `/v2/users` |
| 设置“重写为” | 覆盖 proxy_pass URI | 用重写前缀替换 location 前缀 | 重写 `/v2`：`/api/users` → `/v2/users` |

示例：

```text
路径前缀: /api/
目标地址: http://localhost:8080/
重写为:   留空
```

当前端请求 `/api/users` 时，会代理到 `http://localhost:8080/users`。

再如：

```text
路径前缀: /api/
目标地址: http://localhost:8080
重写为:   /v2
```

请求 `/api/users` → `http://localhost:8080/v2/users`（注意是 `/v2/users` 而不是 `/v2users`）。

#### 请求头

生成应用的反向代理会设置：

- `Host` = 上游 host（类似 `$proxy_host`）
- `X-Forwarded-For` / `X-Real-IP`
- `X-Forwarded-Proto`
- `X-Forwarded-Host`（原始 Host）

并配置合理超时，支持长连接与 WebSocket 升级场景。

### 4. 构建生成

确认配置后，先点击“选择保存位置”指定输出路径，再点击“开始构建”，工具会：

1. 复制基础运行壳 `base.exe`
2. 写入图标与版本资源
3. 追加资源 zip（`dist/` + `proxy_config.json` + `app_config.json`）与 footer
4. 写入到已选择的输出路径
5. 配置了签名证书时，在输出 exe 旁生成同名 `-sign.cmd` 签名脚本

构建过程中可以点击“取消构建”中止，已产生的临时文件会自动清理。构建成功后可点击“试运行”直接启动生成的 exe 做冒烟测试。

## 配置持久化与构建方案

- **自动记忆**：界面上修改配置会自动保存到 `%AppData%\deploy-app\last_config.json`，下次启动自动恢复（导入目录、图标路径会重新校验，失效则清空）。
- **命名方案**：在“全局配置 → 构建方案”中可把当前配置保存为命名方案，之后一键载入；支持导出为 JSON 文件分享给团队，或从文件导入。
- 方案内容涵盖全部构建配置（导入目录、应用配置、代理规则、签名等）。**证书密码不在保存范围内**，签名脚本在运行时才输入密码。

## 生成应用的外置配置

生成应用启动时，会优先读取 exe 同目录下的外置配置，存在且为合法 JSON 时覆盖内置配置：

| 文件 | 作用 |
| --- | --- |
| `app_config.json` | 覆盖窗口参数（宽高、标题、全屏、关闭确认、单实例、记住窗口等） |
| `proxy_config.json` | 覆盖全部反向代理规则 |

示例——不发版修改代理目标：

```json
{
  "rules": [
    { "path": "/api/", "target": "http://192.168.1.100:8080/", "rewrite": "", "enabled": true }
  ]
}
```

文件不存在或内容不是合法 JSON 时，自动回退到打包时内置的配置，因此删掉外置文件即可恢复默认行为。

## 命令行构建（CI 集成）

界面“导出方案”得到的 JSON 即为 CLI 配置文件，命令行无界面打包：

```bash
deploy-app build --config 方案.json [--out 输出路径.exe]
```

- `--config` 必填；`--out` 可选，覆盖方案中的输出路径。
- 退出码：成功 `0`，失败 `1`，进度打印到控制台。
- 可嵌入 GitHub Actions / Jenkins 等流水线，实现前端构建后自动打包。

## 平台支持

当前只支持 Windows 10/11 64-bit（base.exe 以 `GOOS=windows GOARCH=amd64` 预编译，签名、窗口状态等能力均基于 Windows API）。未来若需拓展其他平台，主要工作包括：

1. 分别准备对应平台的预编译运行壳（`cmd/build-base` 增加目标平台矩阵），`base_version.txt` 哈希清单机制可直接复用；
2. 调整资源嵌入方式——macOS 向 Mach-O 追加数据会破坏代码签名，需要改为外挂资源包等方案；
3. `internal/nginxproxy`、`internal/resourcefs`、`internal/shellinfo` 均为纯标准库实现，与平台无关，可直接复用。

## 工作原理

Deploy App 内置一个预编译的 Wails 基础运行壳 `base.exe`。构建时不会现场编译用户应用，而是把前端资源和配置追加到 `base.exe` 尾部：

```text
base.exe
  +
resource.zip
  dist/
  proxy_config.json
  app_config.json
  +
footer
  magic:  RESO
  offset: zip 起始位置
```

生成的 exe 启动后会从自身文件末尾读取资源 zip，懒加载 `dist`（不把整个 zip 读进内存），启用 SPA fallback，并启动内置代理。

## 项目结构

```text
.
├── app.go                    # Wails 绑定适配层（薄）
├── assets.go                 # 运行壳嵌入资产 + 源文件哈希计算
├── main.go                   # 入口：GUI 装配 + CLI 模式
├── extract_zip_test.go       # ZIP 导入测试
├── shell_assets_test.go      # 运行壳与模板一致性测试
├── internal/
│   ├── appconf/              # 构建配置结构、校验、运行时配置生成
│   ├── buildkit/             # 打包管线（build/resource/icon/sign），不依赖 Wails
│   ├── settings/             # 配置持久化与构建方案管理
│   ├── nginxproxy/           # 与生成壳共用的 nginx 路径算法
│   ├── resourcefs/           # 与生成壳共用的 zip FS 与 SPA 回退实现
│   └── shellinfo/            # 运行壳源文件清单（build-base 与打包工具共用）
├── cmd/build-base/           # 生成基础运行壳的工具
├── frontend/                 # Vue 前端界面
│   └── src/components/GlobalPanels.vue  # 构建方案 / 设置 / 拖拽导入等全局面板
├── templates/base/           # 预编译基础 exe + 哈希清单（gitignore 例外保留）
├── templates/generated-app/  # 基础运行壳源码模板
├── .github/workflows/        # CI（测试、前端构建、base.exe 一致性）
└── assets/img/               # README 截图资源
```

### 架构分层

- **根包（main）只做装配**：Wails 选项、绑定适配、嵌入资产注入，不含业务逻辑。
- **internal/buildkit** 是打包管线核心，不依赖 Wails：进度通过回调上报、取消通过 context 传递，GUI 与 CLI 共用同一实现。
- **internal/appconf** 承载配置结构与全部校验；**internal/settings** 承载持久化与方案管理；两者均可独立测试。
- **internal/nginxproxy、resourcefs、shellinfo** 为纯标准库实现，与平台无关，由 build-base 自动同步进生成壳。

## 开发注意

- 修改 `templates/generated-app/*.tmpl` 或 `internal/nginxproxy`、`internal/resourcefs` 后必须执行 `go run ./cmd/build-base`。
- `templates/base/base.exe` 在 `.gitignore` 中通过 `!templates/base/base.exe` 例外保留，避免被 `*.exe` 规则忽略。
- `cmd/build-base` 会把 `internal/nginxproxy/path.go` 与 `internal/resourcefs/resourcefs.go` 自动复制进生成壳（仅替换 package 声明），无需手动同步代码；打包工具构建时还会比对 `base_version.txt` 中的哈希，检测“改了模板但没重新生成 base.exe”的漂移。
- 主工程的 `go vet` / `go test` 依赖 `frontend/dist` 目录存在，仓库保留了 `frontend/dist/.gitkeep` 占位，正常构建前端后 vite 会自动补回该文件。
- 前端步骤有前置校验：未导入 dist / 未选择保存位置 / 应用名非法时不能开始构建。

## 常见问题

### 生成的 exe 打开后提示“加载资源失败”

通常是 exe 被损坏、被二次修改，构建过程没有完整写入资源，或 `base.exe` 仍是占位文件。请先 `go run ./cmd/build-base`，再重新构建。

### 打包工具提示“base.exe 已过期/漂移”

说明 `templates/generated-app` 模板或 `internal/nginxproxy`、`internal/resourcefs` 共享源码在 base.exe 生成之后被修改过。执行 `go run ./cmd/build-base` 重新生成运行壳即可。

### 签名后杀毒软件仍报警告

签名解决的是“发布者身份可信”问题；SmartScreen 还需要证书具备一定声誉积累。自签名或新证书初期仍可能被提示，属正常现象。生成应用的代理不生效但配置无误时，可尝试在 exe 同目录放置外置 `proxy_config.json` 验证是否为内置配置问题。

### 反向代理不生效

检查：

1. 路径前缀是否以 `/` 开头，例如 `/api/`。
2. 目标地址是否为 `http://` 或 `https://`。
3. 目标地址末尾是否带 `/`：这会决定是否剥离路径前缀（与 nginx 相同）。
4. “重写为”若填写，必须以 `/` 开头，且会覆盖目标地址中的路径部分。

### 图标没有生效

优先使用标准 `.ico` 文件，或使用 256x256 以上的正方形 `.png`。如果 Windows 资源管理器仍显示旧图标，可能是系统图标缓存导致，可以换一个输出文件名后再查看。

### History 路由刷新 404

生成应用已启用 SPA fallback：无扩展名路径或 `.html` 在资源中不存在时回退 `index.html`；静态资源扩展名（`.js` / `.css` / 图片字体等）仍返回真实 404。

### 是否支持 macOS 或 Linux

当前只支持 Windows。要支持其他平台，需要分别准备对应平台的基础运行壳和资源写入逻辑。

## 贡献

欢迎提交 Issue 和 Pull Request。建议在提交前先说明要解决的问题、使用场景和预期行为，方便保持功能边界清晰。

本项目偏工具型应用，代码修改建议遵循这些原则：

- 优先保持实现简单直接。
- UI 交互以清晰、稳定、可重复操作为主。
- 新功能尽量补充 README 或界面说明。
- 修改模板或共享 internal 包后请重新运行 `go run ./cmd/build-base`。
- 路径代理逻辑变更时，只需修改 `internal/nginxproxy`（生成壳通过 build-base 自动同步），并补充测试用例。

## 许可证

本项目基于 [MIT License](LICENSE) 开源。