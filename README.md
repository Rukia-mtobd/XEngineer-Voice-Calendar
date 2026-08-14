# XEngineer Voice Calendar

基于 Go、Wails v2 和原生 Web 技术实现的 Windows 桌面语音日历。

## 功能

- DashScope 语音识别与自然语言日程解析
- 日程新增、编辑、删除、重要标记和冲突检测
- SQLite 本地持久化与 CSV 导出
- 中文、英文界面和浅色、深色主题
- Windows 原生系统通知
  - 应用启动后每分钟检查一次日程
  - 在有明确开始时间的日程开始前 10 分钟提醒
  - 同一条日程在一次应用运行期间只提醒一次
  - 可在设置中发送测试通知

## 技术栈

- Go 1.25
- Wails v2
- 原生 HTML、CSS 和 JavaScript，无前端框架或打包工具
- GORM + SQLite
- DashScope `qwen3-asr-flash` 和 `qwen-turbo`
- Windows Toast Notification

## 项目结构

```text
.
├── main.go                         # Wails 入口和生命周期
├── app.go                          # 前后端绑定与业务编排
├── frontend/
│   ├── index.html                  # 页面结构
│   ├── styles/main.css             # 全局样式和组件样式
│   ├── js/app.js                   # 页面状态、渲染和交互
│   └── wailsjs/                    # Wails 自动生成的桥接代码
└── internal/
    ├── asr/                        # DashScope 语音识别
    ├── llm/                        # 日程及意图解析
    ├── storage/                    # SQLite 数据访问
    ├── notification/               # 操作系统通知适配
    ├── reminder/                   # 提醒调度与去重
    ├── config/                     # 本地配置
    └── tracing/                    # OpenTelemetry 本地追踪
```

前端保持零构建设计。Wails 会直接嵌入整个 `frontend` 目录，因此 CSS 和 JavaScript 拆分后不需要额外构建步骤。

## 开发环境

需要：

- Go 1.25+
- Wails CLI v2
- Windows WebView2 Runtime
- 可用的 DashScope API Key

安装 Wails CLI：

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

启动开发模式：

```powershell
wails dev
```

首次启动后，在设置页保存自己的 DashScope API Key。不要把 API Key 写入源码、文档或提交到 Git。

## 测试

运行所有 Go 单元测试：

```powershell
go test ./...
```

当前测试覆盖：

- 时间格式和日期归一化
- LLM JSON 容错解析
- SQLite 日程创建、更新、查询、重要标记和删除
- 跨午夜时长计算
- 系统提醒窗口、去重、失败重试和存储错误

运行静态检查：

```powershell
go vet ./...
```

生成或刷新 Wails 桥接代码：

```powershell
wails generate module
```

## 构建

```powershell
wails build -platform windows/amd64
```

默认产物：

```text
build/bin/XEngineer-Voice-Calendar.exe
```

## 本地数据与安全

运行时会产生以下本地文件：

- `config.json`：DashScope API Key
- `voice_calendar.db`：日程数据库
- `voice_calendar_trace.jsonl`：本地诊断追踪

这些文件可能包含密钥或日程隐私数据，必须保持在版本控制之外。发布前还应确认 Git 历史中不存在旧密钥或真实数据库；如果密钥曾被提交，应先在服务端撤销和轮换，再清理 Git 历史。

## 系统通知说明

通知由 Go 后台调度，不要求设置页面或某个日历页面保持打开。应用完全退出后不会继续提醒；如果需要退出后仍可提醒，应进一步接入 Windows Task Scheduler 或注册开机自启动后台进程。

当前非 Windows 构建会返回“系统通知不支持”的明确错误，不会静默模拟通知。
