package main

import (
	"context"
	"fmt"
)

// App 前后端交互载体，后续功能方法在此扩展。
type App struct {
	ctx context.Context
}

// NewApp 创建 App 实例。
func NewApp() *App {
	return &App{}
}

// startup 在应用启动时由 Wails 调用。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// Placeholder 预留的前后端通信占位方法，暂无业务逻辑。
func (a *App) Placeholder() {}

// SaveApiKey 接收前端传入的 API Key，当前仅打印日志。
func (a *App) SaveApiKey(key string) {
	fmt.Println("SaveApiKey called, key length:", len(key))
}
