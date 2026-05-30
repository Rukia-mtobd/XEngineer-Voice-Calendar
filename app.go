package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"xengineer-voice-calendar/internal/asr"
	"xengineer-voice-calendar/internal/config"
	"xengineer-voice-calendar/internal/llm"
	"xengineer-voice-calendar/internal/storage"
)

// App 前后端交互载体，后续功能方法在此扩展。
type App struct {
	ctx   context.Context
	asr   *asr.Client
	llm   *llm.Client
	store *storage.Store
}

// NewApp 创建 App 实例。
func NewApp() *App {
	store, err := storage.NewStore("")
	if err != nil {
		// 初始化失败时直接 panic，避免应用在无持久化能力时静默运行。
		panic(fmt.Errorf("init sqlite store failed: %w", err))
	}

	return &App{
		asr:   asr.NewClient(),
		llm:   llm.NewClient(),
		store: store,
	}
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

// RecognizeSpeech 将 Base64 音频识别为文字。
func (a *App) RecognizeSpeech(audioBase64, mimeType, format string) (string, error) {
	apiKey, err := config.LoadApiKey()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(apiKey) == "" {
		return "", errors.New("请先配置并保存阿里云 API Key")
	}
	return a.asr.Recognize(apiKey, audioBase64, mimeType, format)
}

// ParseSchedule 调用大模型将语音识别文本解析为结构化日程列表。
func (a *App) ParseSchedule(text string) ([]llm.ParsedSchedule, error) {
	apiKey, err := config.LoadApiKey()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("请先配置并保存阿里云 API Key")
	}
	return a.llm.ParseSchedule(apiKey, text, time.Now())
}

// CreateSchedule 新增一条日程到 SQLite。
func (a *App) CreateSchedule(item llm.ParsedSchedule) (storage.ScheduleRecord, error) {
	rec := storage.ScheduleRecord{
		Date:      strings.TrimSpace(item.Date),
		Title:     strings.TrimSpace(item.Title),
		StartTime: strings.TrimSpace(item.StartTime),
		EndTime:   strings.TrimSpace(item.EndTime),
		Duration:  item.Duration,
		Desc:      strings.TrimSpace(item.Desc),
	}
	if rec.Date == "" {
		return storage.ScheduleRecord{}, errors.New("date 不能为空")
	}
	if rec.Title == "" {
		return storage.ScheduleRecord{}, errors.New("title 不能为空")
	}
	return a.store.CreateSchedule(rec)
}

// ListSchedules 查询全部日程。
func (a *App) ListSchedules() ([]storage.ScheduleRecord, error) {
	return a.store.ListSchedules()
}

// ListSchedulesByDate 按日期查询日程。
func (a *App) ListSchedulesByDate(date string) ([]storage.ScheduleRecord, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return nil, errors.New("date 不能为空")
	}
	return a.store.ListSchedulesByDate(date)
}
