package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"xengineer-voice-calendar/internal/tracing"
)

// DeleteIntent 是删除语义识别结果。
// action 仅为 delete 或 none。
type DeleteIntent struct {
	Action string `json:"action"`
	ID     uint   `json:"id"`
	Date   string `json:"date"`
	Title  string `json:"title"`
}

func (c *Client) ParseDeleteIntent(ctx context.Context, apiKey, text string, ref time.Time) (DeleteIntent, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return DeleteIntent{Action: "none"}, nil
	}
	if shouldIgnoreAsCreateForDelete(text) {
		tracing.Event(ctx, "llm.delete_skipped", map[string]string{
			"reason": "create_keyword_guard",
			"text":   text,
		})
		return DeleteIntent{Action: "none"}, nil
	}

	systemPrompt := buildDeleteIntentPrompt(ref)
	content, _, err := c.invokeLLM(ctx, "llm.ParseDeleteIntent", apiKey, systemPrompt, text)
	if err != nil {
		return DeleteIntent{}, err
	}
	if strings.TrimSpace(content) == "" {
		return DeleteIntent{Action: "none"}, nil
	}

	content = stripMarkdownFence(content)

	var intent DeleteIntent
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		tracing.RecordError(ctx, err)
		return DeleteIntent{}, fmt.Errorf("解析删除意图 JSON 失败: %w", err)
	}

	intent.Action = strings.TrimSpace(strings.ToLower(intent.Action))
	intent.Date = normalizeDate(intent.Date)
	intent.Title = strings.TrimSpace(intent.Title)
	if intent.Action != "delete" {
		intent.Action = "none"
	}

	if b, mErr := json.Marshal(intent); mErr == nil {
		tracing.Event(ctx, "llm.delete_intent_result", map[string]string{
			"intent": string(b),
		})
	}

	return intent, nil
}

func shouldIgnoreAsCreateForDelete(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	if normalized == "" {
		return false
	}
	normalized = strings.ReplaceAll(normalized, " ", "")

	createHints := []string{
		"新建", "新增", "创建", "添加", "加个", "安排", "记一下", "记一个",
		"add", "create", "new",
	}
	deleteHints := []string{
		"删除", "删掉", "移除", "去掉", "取消", "清空",
		"delete", "remove", "cancel",
	}

	hasCreate := false
	for _, kw := range createHints {
		if strings.Contains(normalized, kw) {
			hasCreate = true
			break
		}
	}
	if !hasCreate {
		return false
	}

	for _, kw := range deleteHints {
		if strings.Contains(normalized, kw) {
			return false
		}
	}
	return true
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

func stripMarkdownFence(content string) string {
	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}
	return content
}
