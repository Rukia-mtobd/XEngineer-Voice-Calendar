package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"xengineer-voice-calendar/internal/config"
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

// SaveApiKey 接收前端传入的 API Key 并保存到本地 config.json。
func (a *App) SaveApiKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return errors.New("API Key不能为空，请输入")
	}

	if err := config.SaveApiKey(key); err != nil {
		fmt.Println("SaveApiKey error:", err)
		return err
	}

	fmt.Println("SaveApiKey called, key length:", len(key))
	return nil
}

// LoadApiKey 从本地 config.json 读取 API Key。
func (a *App) LoadApiKey() string {
	key, err := config.LoadApiKey()
	if err != nil {
		fmt.Println("LoadApiKey error:", err)
		return ""
	}
	fmt.Println("LoadApiKey called, key length:", len(key))
	return key
}
