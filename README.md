# 语音日历 (Voice Calendar)

基于 **Go + Wails v2** 的 Windows 桌面应用：通过语音创建日程，本地 SQLite 存储，到期桌面提醒。

## 功能

- 阿里云 FUN-ASR 语音识别（`fun-asr-realtime`）
- 中文自然语言日程解析（如「明天下午3点开会」）
- 日历视图与日程 CRUD
- 本地 API Key 配置与持久化
- 后台定时提醒弹窗

## 技术栈

| 层级 | 技术 |
|------|------|
| 桌面框架 | Wails v2 |
| 后端 | Go |
| 前端 | HTML + JavaScript |
| 数据库 | SQLite |
| 语音 | 阿里云 DashScope FUN-ASR |

## 环境要求

- Go 1.22+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- Windows 10/11（打包目标平台）
- 阿里云百炼 [DashScope API Key](https://help.aliyun.com/zh/model-studio/get-api-key)

## 开发运行

```bash
# 安装 Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# 进入项目目录
cd XEngineer-Voice-Calendar

# 下载依赖
go mod tidy

# 开发模式（热重载）
wails dev
```

## 打包 Windows EXE

```bash
wails build -platform windows/amd64
```

产物位于 `build/bin/VoiceCalendar.exe`，绿色免安装，双击运行。

## 项目结构

```
├── main.go              # Wails 入口
├── app.go               # 前后端绑定方法
├── wails.json           # 打包配置
├── frontend/
│   └── index.html       # 前端页面
└── internal/
    ├── config/          # API Key 管理
    ├── asr/             # 阿里云语音识别
    ├── parser/          # 日程文本解析
    ├── calendar/        # 日程 CRUD
    ├── reminder/        # 定时提醒
    └── db/              # SQLite 数据层
```

## 使用说明

1. 首次启动在顶部输入并保存阿里云 DashScope API Key
2. 按住「按住说话」按钮录音，松开后自动识别并创建日程
3. 也可在右侧手动添加日程
4. 日程到期时弹出提醒窗口

## 数据存储

配置与日程保存在用户目录：

```
%APPDATA%\XEngineer-Voice-Calendar\calendar.db
```
