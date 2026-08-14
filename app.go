package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
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
	"xengineer-voice-calendar/internal/notification"
	"xengineer-voice-calendar/internal/reminder"
	"xengineer-voice-calendar/internal/storage"
	"xengineer-voice-calendar/internal/tracing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 前后端交互载体，后续功能方法在此扩展。
type App struct {
	ctx             context.Context
	asr             *asr.Client
	llm             *llm.Client
	store           *storage.Store
	notifier        notification.Notifier
	notifierInitErr error
	reminderCancel  context.CancelFunc
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

	notifier, notifierErr := notification.New()
	return &App{
		asr:             asr.NewClient(),
		llm:             llm.NewClient(),
		store:           store,
		notifier:        notifier,
		notifierInitErr: notifierErr,
	}
}

// startup 在应用启动时由 Wails 调用。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if err := tracing.Init(); err != nil {
		fmt.Println("tracing init failed:", err)
	}
	if a.notifierInitErr != nil {
		fmt.Println("system notifications unavailable:", a.notifierInitErr)
		return
	}
	reminderCtx, cancel := context.WithCancel(ctx)
	a.reminderCancel = cancel
	service := reminder.New(a.store, a.notifier, reminder.DefaultLeadTime, reminder.DefaultPollInterval)
	go service.Run(reminderCtx)
}

func (a *App) shutdown(ctx context.Context) {
	if a.reminderCancel != nil {
		a.reminderCancel()
	}
	if err := tracing.Shutdown(ctx); err != nil {
		fmt.Println("tracing shutdown failed:", err)
	}
	if err := a.store.Close(); err != nil {
		fmt.Println("sqlite shutdown failed:", err)
	}
}

// SendTestNotification verifies that native system notifications are available.
func (a *App) SendTestNotification() error {
	if a.notifierInitErr != nil {
		return fmt.Errorf("系统通知不可用: %w", a.notifierInitErr)
	}
	if a.notifier == nil {
		return errors.New("系统通知未初始化")
	}
	return a.notifier.Send("语音日历", "系统通知已成功启用")
}

func (a *App) traceCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
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
	ctx, span := tracing.StartSpan(a.traceCtx(), "app.ParseSchedule")
	defer span.End()

	apiKey, err := config.LoadApiKey()
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		err = errors.New("请先配置并保存阿里云 API Key")
		tracing.RecordError(ctx, err)
		return nil, err
	}

	tracing.SetAttrs(ctx, map[string]string{"input.text": text})
	out, err := a.llm.ParseSchedule(ctx, apiKey, text, time.Now())
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, err
	}
	if b, mErr := json.Marshal(out); mErr == nil {
		tracing.Event(ctx, "app.parse_schedule.done", map[string]string{
			"result": string(b),
			"count":  fmt.Sprintf("%d", len(out)),
		})
	}
	return out, nil
}

// ParseDeleteIntent 调用大模型识别删除意图与匹配条件。
func (a *App) ParseDeleteIntent(text string) (llm.DeleteIntent, error) {
	ctx, span := tracing.StartSpan(a.traceCtx(), "app.ParseDeleteIntent")
	defer span.End()

	apiKey, err := config.LoadApiKey()
	if err != nil {
		tracing.RecordError(ctx, err)
		return llm.DeleteIntent{}, err
	}
	if strings.TrimSpace(apiKey) == "" {
		err = errors.New("请先配置并保存阿里云 API Key")
		tracing.RecordError(ctx, err)
		return llm.DeleteIntent{}, err
	}
	tracing.SetAttrs(ctx, map[string]string{"input.text": text})
	out, err := a.llm.ParseDeleteIntent(ctx, apiKey, text, time.Now())
	if err != nil {
		tracing.RecordError(ctx, err)
		return llm.DeleteIntent{}, err
	}
	if b, mErr := json.Marshal(out); mErr == nil {
		tracing.Event(ctx, "app.parse_delete_intent.done", map[string]string{"intent": string(b)})
	}
	return out, nil
}

// ParseUpdateIntent 调用大模型识别修改意图与更新字段。
func (a *App) ParseUpdateIntent(text string) (llm.UpdateIntent, error) {
	ctx, span := tracing.StartSpan(a.traceCtx(), "app.ParseUpdateIntent")
	defer span.End()

	apiKey, err := config.LoadApiKey()
	if err != nil {
		tracing.RecordError(ctx, err)
		return llm.UpdateIntent{}, err
	}
	if strings.TrimSpace(apiKey) == "" {
		err = errors.New("请先配置并保存阿里云 API Key")
		tracing.RecordError(ctx, err)
		return llm.UpdateIntent{}, err
	}
	tracing.SetAttrs(ctx, map[string]string{"input.text": text})
	out, err := a.llm.ParseUpdateIntent(ctx, apiKey, text, time.Now())
	if err != nil {
		tracing.RecordError(ctx, err)
		return llm.UpdateIntent{}, err
	}
	if b, mErr := json.Marshal(out); mErr == nil {
		tracing.Event(ctx, "app.parse_update_intent.done", map[string]string{"intent": string(b)})
	}
	return out, nil
}

// ParseControlIntent 调用大模型识别界面控制意图。
func (a *App) ParseControlIntent(text string) (llm.ControlIntent, error) {
	ctx, span := tracing.StartSpan(a.traceCtx(), "app.ParseControlIntent")
	defer span.End()

	apiKey, err := config.LoadApiKey()
	if err != nil {
		tracing.RecordError(ctx, err)
		return llm.ControlIntent{}, err
	}
	if strings.TrimSpace(apiKey) == "" {
		err = errors.New("请先配置并保存阿里云 API Key")
		tracing.RecordError(ctx, err)
		return llm.ControlIntent{}, err
	}
	tracing.SetAttrs(ctx, map[string]string{"input.text": text})
	out, err := a.llm.ParseControlIntent(ctx, apiKey, text, time.Now())
	if err != nil {
		tracing.RecordError(ctx, err)
		return llm.ControlIntent{}, err
	}
	if b, mErr := json.Marshal(out); mErr == nil {
		tracing.Event(ctx, "app.parse_control_intent.done", map[string]string{"intent": string(b)})
	}
	return out, nil
}

// CreateSchedule 新增一条日程到 SQLite。
func (a *App) CreateSchedule(item llm.ParsedSchedule) (storage.ScheduleRecord, error) {
	ctx, span := tracing.StartSpan(a.traceCtx(), "app.CreateSchedule")
	defer span.End()

	if b, err := json.Marshal(item); err == nil {
		tracing.Event(ctx, "app.create_schedule.input", map[string]string{"schedule": string(b)})
	}

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
		err := errors.New("title 不能为空")
		tracing.RecordError(ctx, err)
		return storage.ScheduleRecord{}, err
	}
	out, err := a.store.CreateSchedule(rec)
	if err != nil {
		tracing.RecordError(ctx, err)
		return storage.ScheduleRecord{}, err
	}
	if b, mErr := json.Marshal(out); mErr == nil {
		tracing.Event(ctx, "app.create_schedule.done", map[string]string{"record": string(b)})
	}
	return out, nil
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

// ListImportantSchedules 查询全部重要日程。
func (a *App) ListImportantSchedules() ([]storage.ScheduleRecord, error) {
	return a.store.ListImportantSchedules()
}

// ListScheduledDatesByMonth 查询某月有日程的日期（YYYY-MM-DD）。
func (a *App) ListScheduledDatesByMonth(month string) ([]string, error) {
	month = strings.TrimSpace(month)
	if !regexp.MustCompile(`^\d{4}-\d{2}$`).MatchString(month) {
		return nil, errors.New("month 格式必须为 YYYY-MM")
	}
	return a.store.ListScheduledDatesByMonth(month)
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

// DeleteScheduleByDateAndStartTime 按日期+开始时间删除日程。
func (a *App) DeleteScheduleByDateAndStartTime(date, startTime string) (int64, error) {
	date = strings.TrimSpace(date)
	startTime = strings.TrimSpace(startTime)
	if date == "" {
		return 0, errors.New("date 不能为空")
	}
	if startTime == "" {
		return 0, errors.New("startTime 不能为空")
	}
	return a.store.DeleteScheduleByDateAndStartTime(date, startTime)
}

// DeleteScheduleByDateTitleAndStartTime 按日期+标题+开始时间删除日程。
func (a *App) DeleteScheduleByDateTitleAndStartTime(date, title, startTime string) (int64, error) {
	date = strings.TrimSpace(date)
	title = strings.TrimSpace(title)
	startTime = strings.TrimSpace(startTime)
	if date == "" {
		return 0, errors.New("date 不能为空")
	}
	if title == "" {
		return 0, errors.New("title 不能为空")
	}
	if startTime == "" {
		return 0, errors.New("startTime 不能为空")
	}
	return a.store.DeleteScheduleByDateTitleAndStartTime(date, title, startTime)
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

// UpdateScheduleByIDWithDesc 按 ID 更新标题、开始时间、结束时间和备注（结束时间可空）。
func (a *App) UpdateScheduleByIDWithDesc(id uint, title, startTime, endTime, desc string) (storage.ScheduleRecord, error) {
	if id == 0 {
		return storage.ScheduleRecord{}, errors.New("id 不能为空")
	}
	title = strings.TrimSpace(title)
	startTime = strings.TrimSpace(startTime)
	endTime = strings.TrimSpace(endTime)
	desc = strings.TrimSpace(desc)

	if title == "" {
		return storage.ScheduleRecord{}, errors.New("title 不能为空")
	}
	if startTime != "" && !validHHMM(startTime) {
		return storage.ScheduleRecord{}, errors.New("startTime 格式必须为 HH:mm")
	}
	if endTime != "" && !validHHMM(endTime) {
		return storage.ScheduleRecord{}, errors.New("endTime 格式必须为 HH:mm")
	}
	return a.store.UpdateScheduleByIDWithDesc(id, title, startTime, endTime, desc)
}

// SetScheduleImportant 设置某条日程是否为重要日程。
func (a *App) SetScheduleImportant(id uint, important bool) (storage.ScheduleRecord, error) {
	if id == 0 {
		return storage.ScheduleRecord{}, errors.New("id 不能为空")
	}
	return a.store.SetScheduleImportant(id, important)
}

func validHHMM(s string) bool {
	if !regexp.MustCompile(`^\d{2}:\d{2}$`).MatchString(s) {
		return false
	}
	_, err := time.Parse("15:04", s)
	return err == nil
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
