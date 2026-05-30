package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"xengineer-voice-calendar/internal/asr"
	"xengineer-voice-calendar/internal/config"
	"xengineer-voice-calendar/internal/llm"
	"xengineer-voice-calendar/internal/storage"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 前后端交互载体，后续功能方法在此扩展。
type App struct {
	ctx   context.Context
	asr   *asr.Client
	llm   *llm.Client
	store *storage.Store
}

// ExportScheduleItem 是导出 CSV 时前端传入的行数据。
type ExportScheduleItem struct {
	Date      string `json:"date"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Title     string `json:"title"`
	Status    string `json:"status"`
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

// ParseDeleteIntent 调用大模型识别删除意图与匹配条件。
func (a *App) ParseDeleteIntent(text string) (llm.DeleteIntent, error) {
	apiKey, err := config.LoadApiKey()
	if err != nil {
		return llm.DeleteIntent{}, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return llm.DeleteIntent{}, errors.New("请先配置并保存阿里云 API Key")
	}
	return a.llm.ParseDeleteIntent(apiKey, text, time.Now())
}

// ParseUpdateIntent 调用大模型识别修改意图与更新字段。
func (a *App) ParseUpdateIntent(text string) (llm.UpdateIntent, error) {
	apiKey, err := config.LoadApiKey()
	if err != nil {
		return llm.UpdateIntent{}, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return llm.UpdateIntent{}, errors.New("请先配置并保存阿里云 API Key")
	}
	return a.llm.ParseUpdateIntent(apiKey, text, time.Now())
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

// DeleteScheduleByID 按 ID 删除单条日程。
func (a *App) DeleteScheduleByID(id uint) error {
	if id == 0 {
		return errors.New("id 不能为空")
	}
	return a.store.DeleteScheduleByID(id)
}

// DeleteScheduleByDateAndTitle 按日期+标题删除日程。
func (a *App) DeleteScheduleByDateAndTitle(date, title string) (int64, error) {
	date = strings.TrimSpace(date)
	title = strings.TrimSpace(title)
	if date == "" {
		return 0, errors.New("date 不能为空")
	}
	if title == "" {
		return 0, errors.New("title 不能为空")
	}
	return a.store.DeleteScheduleByDateAndTitle(date, title)
}

// DeleteSchedulesByDate 按日期删除全部日程。
func (a *App) DeleteSchedulesByDate(date string) (int64, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return 0, errors.New("date 不能为空")
	}
	return a.store.DeleteSchedulesByDate(date)
}

// UpdateScheduleByID 按 ID 更新标题、开始时间、结束时间（结束时间可空）。
func (a *App) UpdateScheduleByID(id uint, title, startTime, endTime string) (storage.ScheduleRecord, error) {
	if id == 0 {
		return storage.ScheduleRecord{}, errors.New("id 不能为空")
	}
	title = strings.TrimSpace(title)
	startTime = strings.TrimSpace(startTime)
	endTime = strings.TrimSpace(endTime)

	if title == "" {
		return storage.ScheduleRecord{}, errors.New("title 不能为空")
	}
	if startTime != "" && !validHHMM(startTime) {
		return storage.ScheduleRecord{}, errors.New("startTime 格式必须为 HH:mm")
	}
	if endTime != "" && !validHHMM(endTime) {
		return storage.ScheduleRecord{}, errors.New("endTime 格式必须为 HH:mm")
	}
	return a.store.UpdateScheduleByID(id, title, startTime, endTime)
}

func validHHMM(s string) bool {
	return regexp.MustCompile(`^\d{2}:\d{2}$`).MatchString(s)
}

// ExportSchedulesCSV 导出当前日程列表为 CSV 文件。
// 前端传入当前筛选后的数据，后端通过系统保存对话框让用户选择路径。
func (a *App) ExportSchedulesCSV(items []ExportScheduleItem) (string, error) {
	if a.ctx == nil {
		return "", errors.New("应用上下文未初始化")
	}
	if len(items) == 0 {
		return "", errors.New("没有可导出的日程")
	}

	filename := fmt.Sprintf("voice-calendar-%s.csv", time.Now().Format("20060102-150405"))
	targetPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "导出日程为 CSV",
		DefaultFilename: filename,
		Filters: []runtime.FileFilter{
			{
				DisplayName: "CSV Files (*.csv)",
				Pattern:     "*.csv",
			},
		},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(targetPath) == "" {
		return "", errors.New("已取消导出")
	}
	if filepath.Ext(strings.ToLower(targetPath)) != ".csv" {
		targetPath += ".csv"
	}

	f, err := os.Create(targetPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"日期", "开始时间", "结束时间", "标题", "状态"}); err != nil {
		return "", err
	}
	for _, it := range items {
		if err := w.Write([]string{
			strings.TrimSpace(it.Date),
			strings.TrimSpace(it.StartTime),
			strings.TrimSpace(it.EndTime),
			strings.TrimSpace(it.Title),
			strings.TrimSpace(it.Status),
		}); err != nil {
			return "", err
		}
	}
	if err := w.Error(); err != nil {
		return "", err
	}
	return targetPath, nil
}
