package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const generationURL = "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation"
const defaultModel = "qwen3.6-flash"

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
func (c *Client) ParseSchedule(apiKey, text string, ref time.Time) ([]ParsedSchedule, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("输入文本为空")
	}

	systemPrompt := buildSystemPrompt(ref)
	reqBody := chatRequest{Model: defaultModel}
	reqBody.Input.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: text},
	}
	reqBody.Parameters.ResultFormat = "message"

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, generationURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用大模型接口失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("大模型接口返回 %d: %s", resp.StatusCode, string(raw))
	}

	var result chatResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("解析大模型响应失败: %w", err)
	}
	if result.Code != "" && result.Code != "Success" {
		return nil, fmt.Errorf("大模型调用失败: %s", result.Message)
	}

	content := ""
	if len(result.Output.Choices) > 0 {
		content = extractMessageContent(result.Output.Choices[0].Message.Content)
	} else {
		content = strings.TrimSpace(result.Output.Text)
	}
	if strings.TrimSpace(content) == "" {
		return nil, fmt.Errorf("大模型返回内容为空")
	}

	schedules, err := decodeScheduleJSONArray(content, text, ref)
	if err != nil {
		return nil, fmt.Errorf("解析日程 JSON 失败: %w", err)
	}
	if len(schedules) == 0 {
		return []ParsedSchedule{fallbackSchedule(text, ref)}, nil
	}

	normalized := make([]ParsedSchedule, 0, len(schedules))
	for _, item := range schedules {
		normalized = append(normalized, normalizeSchedule(item, text, ref))
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

	return fmt.Sprintf(`你是日程解析助手。请从用户的中文输入中提取日程信息，并只输出 JSON 数组（不要 markdown、不要解释）。

当前参考日期：%s（星期%s）

输出格式（数组，可包含 1 条或多条）：
[
  {
    "title": "日程标题",
    "date": "YYYY-MM-DD 或 null",
    "startTime": "HH:mm 或 null",
    "endTime": "HH:mm 或 null",
    "duration": 120,
    "desc": "该条日程对应的原文片段"
  }
]

规则：
1. 一句话包含多个任务时必须拆成多条独立日程，禁止合并（如“明天开会，后天健身”→2条）
2. 支持时间段解析（如“下午3点到5点”→ startTime=15:00, endTime=17:00, duration=120）
3. 有 startTime 和 endTime 时，duration 为持续分钟数，需自动计算
4. 仅有开始时间时，endTime 填 null，duration 可为 0 或 null
5. 将“今天/明天/后天/下周一/5月30日”等转为 YYYY-MM-DD；无法识别填 null
6. 时间统一为 24 小时制 HH:mm；无法识别填 null
7. title 为核心事项；无法提取时用 desc 或原文
8. desc 保留该条日程相关的原文片段；单条时可用完整原文
9. 即使缺少日期或时间，也要生成日程条目，不要报错`, today, weekday)
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

	item.Date = normalizeDate(item.Date)
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
