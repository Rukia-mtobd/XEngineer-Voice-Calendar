package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"xengineer-voice-calendar/internal/tracing"
)

const generationURL = "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"
const defaultModel = "qwen-turbo"

// ParsedSchedule 大模型解析后的单条结构化日程。
type ParsedSchedule struct {
	Title     string `json:"title"`
	Date      string `json:"date"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Duration  int    `json:"duration"`
	Desc      string `json:"desc"`
}

type Client struct {
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

type chatRequest struct {
	Model string `json:"model"`
	Input struct {
		Messages []chatMessage `json:"messages"`
	} `json:"input"`
	Parameters struct {
		ResultFormat string `json:"result_format"`
	} `json:"parameters"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Output struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Text string `json:"text"`
	} `json:"output"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type rawSchedule struct {
	Title     *string `json:"title"`
	Date      any     `json:"date"`
	StartTime any     `json:"startTime"`
	EndTime   any     `json:"endTime"`
	Time      any     `json:"time"`
	Duration  any     `json:"duration"`
	Desc      *string `json:"desc"`
}

// ParseSchedule 调用通义千问，将自然语言文本解析为结构化日程列表。
func (c *Client) ParseSchedule(ctx context.Context, apiKey, text string, ref time.Time) ([]ParsedSchedule, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("输入文本为空")
	}

	tracing.SetAttrs(ctx, map[string]string{
		"ref_date": ref.Format("2006-01-02"),
	})

	systemPrompt := buildSystemPrompt(ref)
	content, _, err := c.invokeLLM(ctx, "llm.ParseSchedule", apiKey, systemPrompt, text)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		err = fmt.Errorf("大模型返回内容为空")
		tracing.RecordError(ctx, err)
		return nil, err
	}

	jsonStr := extractJSONArray(content)
	tracing.Event(ctx, "llm.json_extracted", map[string]string{
		"json_array": jsonStr,
	})

	schedules, err := decodeScheduleJSONArray(content, text, ref)
	if err != nil {
		tracing.RecordError(ctx, err)
		return nil, fmt.Errorf("解析日程 JSON 失败: %w", err)
	}

	if b, mErr := json.Marshal(schedules); mErr == nil {
		tracing.Event(ctx, "llm.decoded_schedules", map[string]string{
			"items": string(b),
			"count": fmt.Sprintf("%d", len(schedules)),
		})
	}

	if len(schedules) == 0 {
		fb := fallbackSchedule(text, ref)
		tracing.Event(ctx, "llm.fallback_schedule", map[string]string{
			"title": fb.Title,
			"date":  fb.Date,
		})
		return []ParsedSchedule{fb}, nil
	}

	normalized := make([]ParsedSchedule, 0, len(schedules))
	for i, item := range schedules {
		before := item
		norm := normalizeSchedule(item, text, ref)
		normalized = append(normalized, norm)
		tracing.Event(ctx, "llm.normalize_schedule", map[string]string{
			"index":        fmt.Sprintf("%d", i),
			"before_date":  before.Date,
			"after_date":   norm.Date,
			"before_title": before.Title,
			"after_title":  norm.Title,
			"start_time":   norm.StartTime,
			"end_time":     norm.EndTime,
		})
	}

	if b, mErr := json.Marshal(normalized); mErr == nil {
		tracing.Event(ctx, "llm.normalized_result", map[string]string{
			"result": string(b),
		})
	}

	return normalized, nil
}

func extractMessageContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return strings.TrimSpace(text)
	}

	var parts []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var texts []string
		for _, part := range parts {
			if t := strings.TrimSpace(part.Text); t != "" {
				texts = append(texts, t)
			}
		}
		return strings.Join(texts, "")
	}

	return ""
}

func buildSystemPrompt(ref time.Time) string {
	weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
	today := ref.Format("2006-01-02")
	weekday := weekdays[int(ref.Weekday())]

	return fmt.Sprintf(`你是日程解析器，只输出JSON数组，无任何解释。

当前参考日期（今天）：%s（星期%s）

输出格式：[{"title":"事件标题","date":"YYYY-MM-DD","startTime":"HH:mm","endTime":"","duration":0,"desc":""}]

规则：
1. title（必填）：提炼日程核心主题，2-8字为宜；去掉日期、星期、相对时间、具体时刻等时间词，保留事件本身。
   - 「明天下午三点吃饭」→ title="吃饭"
   - 「后天上午十点开会」→ title="开会"
   - 「周五晚上健身」→ title="健身"
   - 「记一下买牛奶」→ title="买牛奶"
   title 禁止留空，禁止把整句原文当作 title。
2. desc：可写补充说明；无补充时可用空字符串，不要把 desc 当作 title 的替代品。
3. startTime：必须从输入提取明确时刻，如「下午三点」→"15:00"；用户未提时间则返回""。
4. date：转为 YYYY-MM-DD；相对日期基于参考日：今天=参考日，明天=+1天，后天=+2天，昨天=-1天。
5. 未提及年份时使用参考日期的年份，禁止默认 2024 或其他历史年份。
6. 无法解析日期时 date="" ；只输出 JSON，不要 markdown 或解释。`, today, weekday)
}

func decodeScheduleJSONArray(content, source string, ref time.Time) ([]ParsedSchedule, error) {
	jsonStr := extractJSONArray(content)

	var rawItems []rawSchedule
	if err := json.Unmarshal([]byte(jsonStr), &rawItems); err != nil {
		var single rawSchedule
		if errSingle := json.Unmarshal([]byte(jsonStr), &single); errSingle != nil {
			return nil, err
		}
		rawItems = []rawSchedule{single}
	}

	items := make([]ParsedSchedule, 0, len(rawItems))
	for _, raw := range rawItems {
		item := ParsedSchedule{
			Title:     stringValue(raw.Title),
			Date:      normalizeDate(anyToString(raw.Date)),
			StartTime: normalizeTime(anyToString(raw.StartTime)),
			EndTime:   normalizeTime(anyToString(raw.EndTime)),
			Duration:  anyToInt(raw.Duration),
			Desc:      stringValue(raw.Desc),
		}
		if item.StartTime == "" {
			item.StartTime = normalizeTime(anyToString(raw.Time))
		}
		if item.Desc == "" {
			item.Desc = source
		}
		items = append(items, item)
	}
	return items, nil
}

func normalizeSchedule(item ParsedSchedule, source string, ref time.Time) ParsedSchedule {
	item.Title = strings.TrimSpace(item.Title)
	if item.Title == "" {
		item.Title = strings.TrimSpace(item.Desc)
	}
	if item.Title == "" {
		item.Title = source
	}

	item.Desc = strings.TrimSpace(item.Desc)
	if item.Desc == "" {
		item.Desc = source
	}

	item.Date = coerceScheduleDate(item.Date, ref)
	if item.Date == "" {
		item.Date = ref.Format("2006-01-02")
	}

	item.StartTime = normalizeTime(item.StartTime)
	item.EndTime = normalizeTime(item.EndTime)

	if item.Duration <= 0 && item.StartTime != "" && item.EndTime != "" {
		item.Duration = calcDurationMinutes(item.StartTime, item.EndTime)
	}

	return item
}

func fallbackSchedule(text string, ref time.Time) ParsedSchedule {
	return ParsedSchedule{
		Title: text,
		Date:  ref.Format("2006-01-02"),
		Desc:  text,
	}
}

func extractJSONArray(content string) string {
	content = strings.TrimSpace(content)
	re := regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		content = strings.TrimSpace(m[1])
	}

	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		return content[start : end+1]
	}

	objStart := strings.Index(content, "{")
	objEnd := strings.LastIndex(content, "}")
	if objStart >= 0 && objEnd > objStart {
		return "[" + content[objStart:objEnd+1] + "]"
	}

	return content
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		s := strings.TrimSpace(val)
		if strings.EqualFold(s, "null") {
			return ""
		}
		return s
	case float64:
		return strings.TrimSpace(fmt.Sprintf("%v", val))
	default:
		return ""
	}
}

func anyToInt(v any) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int(val)
	case int:
		return val
	case string:
		if strings.TrimSpace(val) == "" || strings.EqualFold(val, "null") {
			return 0
		}
		var n int
		fmt.Sscanf(val, "%d", &n)
		return n
	default:
		return 0
	}
}

func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, s); matched {
		return s
	}
	return ""
}

// coerceScheduleDate 校验并修正 LLM 返回的日期（常见误写为 2024 等训练数据年份）。
func coerceScheduleDate(s string, ref time.Time) string {
	s = normalizeDate(s)
	if s == "" {
		return ""
	}
	parsed, err := time.ParseInLocation("2006-01-02", s, ref.Location())
	if err != nil {
		return ""
	}
	refYear := ref.Year()
	// 年份偏离参考年超过 1 年时，保留月日、改用参考年（用户未说年份的场景）
	if parsed.Year() < refYear-1 || parsed.Year() > refYear+1 {
		parsed = time.Date(refYear, parsed.Month(), parsed.Day(), 0, 0, 0, 0, ref.Location())
	}
	return parsed.Format("2006-01-02")
}

func normalizeTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if matched, _ := regexp.MatchString(`^\d{2}:\d{2}$`, s); matched {
		return s
	}
	return ""
}

func calcDurationMinutes(start, end string) int {
	startT, err1 := time.Parse("15:04", start)
	endT, err2 := time.Parse("15:04", end)
	if err1 != nil || err2 != nil {
		return 0
	}
	minutes := int(endT.Sub(startT).Minutes())
	if minutes < 0 {
		minutes += 24 * 60
	}
	return minutes
}
