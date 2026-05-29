package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/asr"
	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/calendar"
	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/config"
	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/db"
	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/parser"
	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/reminder"
)

type App struct {
	ctx      context.Context
	database *db.Database
	config   *config.Service
	calendar *calendar.Service
	asr      *asr.Client
	reminder *reminder.Service
}

func NewApp() *App {
	return &App{asr: asr.NewClient()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	database, err := db.Open()
	if err != nil {
		runtimeLogError(ctx, "数据库初始化失败: "+err.Error())
		return
	}
	a.database = database
	a.config = config.New(database)
	a.calendar = calendar.New(database)
	a.reminder = reminder.New(database)
	go a.reminder.Start(ctx)
}

func (a *App) shutdown(ctx context.Context) {
	if a.database != nil {
		_ = a.database.Close()
	}
}

func runtimeLogError(ctx context.Context, msg string) {
	// 避免循环依赖，startup 阶段仅打印到 stderr
	fmt.Println(msg)
	_ = ctx
}

// GetAPIKey 返回已保存的阿里云 API Key。
func (a *App) GetAPIKey() (string, error) {
	if a.config == nil {
		return "", fmt.Errorf("服务未就绪")
	}
	return a.config.GetAPIKey()
}

// SaveAPIKey 持久化阿里云 API Key。
func (a *App) SaveAPIKey(key string) error {
	if a.config == nil {
		return fmt.Errorf("服务未就绪")
	}
	return a.config.SaveAPIKey(key)
}

// RecognizeSpeech 将 Base64 音频识别为文字（不创建日程）。
func (a *App) RecognizeSpeech(audioBase64, mimeType, format string) (string, error) {
	apiKey, err := a.config.GetAPIKey()
	if err != nil {
		return "", err
	}
	return a.asr.Recognize(apiKey, audioBase64, mimeType, format)
}

// VoiceCreateEvent 语音识别后自动解析并创建日程。
func (a *App) VoiceCreateEvent(audioBase64, mimeType, format string) (map[string]any, error) {
	text, err := a.RecognizeSpeech(audioBase64, mimeType, format)
	if err != nil {
		return nil, err
	}
	event, err := a.calendar.CreateFromText(text)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"text":  text,
		"event": event,
	}, nil
}

// ParseText 解析文本中的日程信息（调试用）。
func (a *App) ParseText(text string) (map[string]string, error) {
	parsed, err := parser.Parse(text, time.Now())
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"title":       parsed.Title,
		"scheduledAt": parsed.ScheduledAt.Format(time.RFC3339),
	}, nil
}

// ListEvents 返回全部日程。
func (a *App) ListEvents() ([]db.Event, error) {
	if a.calendar == nil {
		return nil, fmt.Errorf("服务未就绪")
	}
	return a.calendar.List()
}

// ListEventsByDate 返回指定日期的日程。
func (a *App) ListEventsByDate(date string) ([]db.Event, error) {
	if a.calendar == nil {
		return nil, fmt.Errorf("服务未就绪")
	}
	return a.calendar.ListByDate(date)
}

// CreateEvent 手动创建日程。
func (a *App) CreateEvent(title, scheduledAt string) (*db.Event, error) {
	if a.calendar == nil {
		return nil, fmt.Errorf("服务未就绪")
	}
	return a.calendar.Create(title, scheduledAt)
}

// UpdateEvent 更新日程。
func (a *App) UpdateEvent(id int64, title, scheduledAt string) (*db.Event, error) {
	if a.calendar == nil {
		return nil, fmt.Errorf("服务未就绪")
	}
	return a.calendar.Update(id, title, scheduledAt)
}

// DeleteEvent 删除日程。
func (a *App) DeleteEvent(id int64) error {
	if a.calendar == nil {
		return fmt.Errorf("服务未就绪")
	}
	return a.calendar.Delete(id)
}
