package main

import (
	"context"
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
