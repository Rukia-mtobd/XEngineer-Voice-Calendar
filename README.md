# XEngineer Voice Calendar
视频网站：https://www.bilibili.com/video/BV1bjVQ68Ek9/?vd_source=076850da6201a8755a8e1ec89b4b54cb
基于 `Go + Wails v2` 的桌面语音日历应用。支持语音转文字、自然语言日程解析、日程新增/删除/更新、本地 SQLite 存储与导出 CSV。

## 功能概览

- 语音输入并识别文本（阿里云 DashScope ASR）
- 日程语义解析（日期/时间/标题/时长）
- 语音意图识别（新增、删除、修改、界面控制）
- 日程管理（CRUD、重要标记、按月有日程日期查询）
- CSV 导出

## 技术栈

- 桌面框架：`Wails v2`
- 后端语言：`Go 1.22`
- 前端：原生 `HTML + CSS + JavaScript`（无前端框架）
- 本地数据库：`SQLite`（通过 `GORM` + `glebarez/sqlite`）
- 语音识别模型：DashScope `qwen3-asr-flash`
- 语义解析模型：DashScope `qwen3.6-flash`

## 外部依赖

### 运行时外部服务

- 阿里云 DashScope API（语音识别与文本解析）
- 你需要可用的 DashScope API Key

### Go 直接依赖（`go.mod`）

- `github.com/wailsapp/wails/v2 v2.12.0`
- `github.com/glebarez/sqlite v1.11.0`
- `gorm.io/gorm v1.31.1`

### Go 间接依赖（`go.mod` 标记为 indirect）

- `git.sr.ht/~jackmordaunt/go-toast/v2 v2.0.3`
- `github.com/bep/debounce v1.2.1`
- `github.com/dustin/go-humanize v1.0.1`
- `github.com/glebarez/go-sqlite v1.21.2`
- `github.com/go-ole/go-ole v1.3.0`
- `github.com/godbus/dbus/v5 v5.1.0`
- `github.com/google/uuid v1.6.0`
- `github.com/gorilla/websocket v1.5.3`
- `github.com/jchv/go-winloader v0.0.0-20210711035445-715c2860da7e`
- `github.com/jinzhu/inflection v1.0.0`
- `github.com/jinzhu/now v1.1.5`
- `github.com/labstack/echo/v4 v4.13.3`
- `github.com/labstack/gommon v0.4.2`
- `github.com/leaanthony/go-ansi-parser v1.6.1`
- `github.com/leaanthony/gosod v1.0.4`
- `github.com/leaanthony/slicer v1.6.0`
- `github.com/leaanthony/u v1.1.1`
- `github.com/mattn/go-colorable v0.1.13`
- `github.com/mattn/go-isatty v0.0.20`
- `github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c`
- `github.com/pkg/errors v0.9.1`
- `github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec`
- `github.com/rivo/uniseg v0.4.7`
- `github.com/samber/lo v1.49.1`
- `github.com/tkrajina/go-reflector v0.5.8`
- `github.com/valyala/bytebufferpool v1.0.0`
- `github.com/valyala/fasttemplate v1.2.2`
- `github.com/wailsapp/go-webview2 v1.0.22`
- `github.com/wailsapp/mimetype v1.4.1`
- `golang.org/x/crypto v0.33.0`
- `golang.org/x/net v0.35.0`
- `golang.org/x/sys v0.30.0`
- `golang.org/x/text v0.22.0`
- `modernc.org/libc v1.22.5`
- `modernc.org/mathutil v1.5.0`
- `modernc.org/memory v1.5.0`
- `modernc.org/sqlite v1.23.1`

### 构建工具依赖

- `Go 1.22+`
- `Wails CLI v2`（本地开发与构建）
- Windows 下建议安装 WebView2 Runtime（多数 Win10/Win11 已内置）

## 快速开始（开发运行）

```bash
# 1) 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 2) 进入项目目录
cd XEngineer-Voice-Calendar

# 3) 下载并整理依赖
go mod tidy

# 4) 启动开发模式（热重载）
wails dev
```

启动后首次使用请在应用设置中保存 DashScope API Key。

## 打包与部署

### 本地构建（Windows）

```bash
wails build -platform windows/amd64
```

默认产物位于 `build/bin/XEngineer-Voice-Calendar.exe`。

### 部署方式（推荐）

- 直接分发 `build/bin` 目录中的可执行文件及其同级资源文件（便携式部署）
- 首次启动后在程序目录旁生成/读取：
  - `config.json`（保存 API Key）
  - `voice_calendar.db`（SQLite 数据文件）

### 生产发布建议

- 使用独立的发布目录，避免开发环境脏文件混入
- 按版本归档发布包（例如 `XEngineer-Voice-Calendar-v0.1.0-win64.zip`）
- 对外分发前做一次离线启动自检（无源码环境）

## 目录结构（当前实现）

```text
.
├─ main.go                    # Wails 入口
├─ app.go                     # 前后端绑定方法
├─ wails.json                 # Wails 构建配置
├─ frontend/
│  ├─ index.html              # 前端页面（UI + 交互）
│  └─ wailsjs/                # Wails 生成的桥接代码
└─ internal/
   ├─ asr/                    # 语音识别调用
   ├─ llm/                    # 日程/意图解析
   ├─ storage/                # SQLite 持久化
   └─ config/                 # config.json 读写
```

## 配置与数据文件
- 可使用key：sk-6efa633396704acfa192ffbef48e21fd
- `config.json`：程序目录下，字段 `api_key`
- `voice_calendar.db`：程序目录下 SQLite 数据库

> 注意：当前实现通过 `os.Executable()` 计算 `config.json` 路径，数据库默认文件名为 `voice_calendar.db`（工作目录解析为绝对路径）。
