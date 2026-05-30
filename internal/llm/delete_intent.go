package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeleteIntent 是删除语义识别结果。
// action 仅为 delete 或 none。
type DeleteIntent struct {
	Action string `json:"action"`
	ID     uint   `json:"id"`
	Date   string `json:"date"`
	Title  string `json:"title"`
}

func (c *Client) ParseDeleteIntent(apiKey, text string, ref time.Time) (DeleteIntent, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return DeleteIntent{Action: "none"}, nil
	}

	systemPrompt := buildDeleteIntentPrompt(ref)
	reqBody := chatRequest{Model: defaultModel}
	reqBody.Input.Messages = []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: text},
	}
	reqBody.Parameters.ResultFormat = "message"

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return DeleteIntent{}, err
	}

	req, err := http.NewRequest(http.MethodPost, generationURL, bytes.NewReader(payload))
	if err != nil {
		return DeleteIntent{}, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DeleteIntent{}, fmt.Errorf("调用大模型接口失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return DeleteIntent{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return DeleteIntent{}, fmt.Errorf("大模型接口返回 %d: %s", resp.StatusCode, string(raw))
	}

	var result chatResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return DeleteIntent{}, fmt.Errorf("解析大模型响应失败: %w", err)
	}
	if result.Code != "" && result.Code != "Success" {
		return DeleteIntent{}, fmt.Errorf("大模型调用失败: %s", result.Message)
	}

	content := ""
	if len(result.Output.Choices) > 0 {
		content = extractMessageContent(result.Output.Choices[0].Message.Content)
	} else {
		content = strings.TrimSpace(result.Output.Text)
	}
	if strings.TrimSpace(content) == "" {
		return DeleteIntent{Action: "none"}, nil
	}

	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var intent DeleteIntent
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		return DeleteIntent{}, fmt.Errorf("解析删除意图 JSON 失败: %w", err)
	}

	intent.Action = strings.TrimSpace(strings.ToLower(intent.Action))
	intent.Date = normalizeDate(intent.Date)
	intent.Title = strings.TrimSpace(intent.Title)
	if intent.Action != "delete" {
		intent.Action = "none"
	}
	return intent, nil
}

func buildDeleteIntentPrompt(ref time.Time) string {
	weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
	today := ref.Format("2006-01-02")
	weekday := weekdays[int(ref.Weekday())]
	return fmt.Sprintf(`你是删除意图识别助手。请只输出 JSON 对象，不要 markdown，不要解释。

当前参考日期：%s（星期%s）

输出格式：
{
  "action": "delete 或 none",
  "id": 0,
  "date": "YYYY-MM-DD 或 空字符串",
  "title": "标题或空字符串"
}

规则：
1) 如果用户意图是删除日程，action=delete，否则 action=none
2) 若用户给出“今天/明天/昨天/后天/具体日期”等，请转换成 YYYY-MM-DD
3) 若用户说“删除今天所有日程”，则 action=delete，date=当天，title=""
4) 若用户说“删除明天下午三点的项目会”，则 action=delete，date=对应日期，title=项目会（可去掉时间词）
5) 未提及 id 时 id=0；未提及标题 title=""；无法识别日期时 date=""
6) 只输出 JSON 对象`, today, weekday)
}
