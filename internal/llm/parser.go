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
const defaultModel = "qwen3.6-flash" // 日程文本解析模型（Qwen3.6 系列需用 multimodal 端点）

// ParsedSchedule 大模型解析后的结构化日程。
type ParsedSchedule struct {
	Title string `json:"title"`
	Date  string `json:"date"`
	Time  string `json:"time"`
	Desc  string `json:"desc"`
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

// ParseSchedule 调用通义千问，将自然语言文本解析为结构化日程。
func (c *Client) ParseSchedule(apiKey, text string, ref time.Time) (ParsedSchedule, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return ParsedSchedule{}, fmt.Errorf("输入文本为空")
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
		return ParsedSchedule{}, err
	}

	req, err := http.NewRequest(http.MethodPost, generationURL, bytes.NewReader(payload))
	if err != nil {
		return ParsedSchedule{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return ParsedSchedule{}, fmt.Errorf("调用大模型接口失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ParsedSchedule{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return ParsedSchedule{}, fmt.Errorf("大模型接口返回 %d: %s", resp.StatusCode, string(raw))
	}

	var result chatResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return ParsedSchedule{}, fmt.Errorf("解析大模型响应失败: %w", err)
	}
	if result.Code != "" && result.Code != "Success" {
		return ParsedSchedule{}, fmt.Errorf("大模型调用失败: %s", result.Message)
	}

	content := ""
	if len(result.Output.Choices) > 0 {
		content = extractMessageContent(result.Output.Choices[0].Message.Content)
	} else {
		content = strings.TrimSpace(result.Output.Text)
	}
	if strings.TrimSpace(content) == "" {
		return ParsedSchedule{}, fmt.Errorf("大模型返回内容为空")
	}

	parsed, err := decodeScheduleJSON(content)
	if err != nil {
		return ParsedSchedule{}, fmt.Errorf("解析日程 JSON 失败: %w", err)
	}

	parsed.Desc = strings.TrimSpace(parsed.Desc)
	if parsed.Desc == "" {
		parsed.Desc = text
	}
	parsed.Title = strings.TrimSpace(parsed.Title)
	if parsed.Title == "" {
		parsed.Title = text
	}
	parsed.Date = normalizeDate(parsed.Date)
	parsed.Time = normalizeTime(parsed.Time)

	return parsed, nil
}

// extractMessageContent 兼容 multimodal 端点返回的 string 或 [{text: "..."}] 格式。
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

	return fmt.Sprintf(`你是日程解析助手。请从用户的中文输入中提取日程信息，并只输出 JSON（不要 markdown、不要解释）。

当前参考日期：%s（星期%s）

输出格式：
{
  "title": "日程标题",
  "date": "YYYY-MM-DD 或 null",
  "time": "HH:mm 或 null",
  "desc": "原始输入文本"
}

规则：
1. 将“今天”“明天”“后天”“下周一”“5月30日”等转换为具体 YYYY-MM-DD
2. 将“下午三点”“15点30分”“晚上8点”等转换为 24 小时制 HH:mm
3. title 为去掉日期时间后的核心事项；无法提取时用原文
4. 无法识别 date 或 time 时填 null
5. desc 保留用户原文`, today, weekday)
}

func decodeScheduleJSON(content string) (ParsedSchedule, error) {
	jsonStr := extractJSONObject(content)

	var raw struct {
		Title *string `json:"title"`
		Date  any     `json:"date"`
		Time  any     `json:"time"`
		Desc  *string `json:"desc"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return ParsedSchedule{}, err
	}

	parsed := ParsedSchedule{
		Title: stringValue(raw.Title),
		Date:  anyToString(raw.Date),
		Time:  anyToString(raw.Time),
		Desc:  stringValue(raw.Desc),
	}
	return parsed, nil
}

func extractJSONObject(content string) string {
	content = strings.TrimSpace(content)
	re := regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		content = strings.TrimSpace(m[1])
	}
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		return content[start : end+1]
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
	default:
		return ""
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
