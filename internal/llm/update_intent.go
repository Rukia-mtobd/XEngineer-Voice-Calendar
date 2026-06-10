package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"xengineer-voice-calendar/internal/tracing"
)

// UpdateIntent 是语音修改意图识别结果。
// action 仅为 update 或 none。
type UpdateIntent struct {
	Action    string `json:"action"`
	ID        uint   `json:"id"`
	Date      string `json:"date"`
	Title     string `json:"title"`     // 目标日程标题（用于匹配）
	NewTitle  string `json:"newTitle"`  // 修改后的标题
	StartTime string `json:"startTime"` // 修改后的开始时间
	EndTime   string `json:"endTime"`   // 修改后的结束时间，可空
}

func (c *Client) ParseUpdateIntent(ctx context.Context, apiKey, text string, ref time.Time) (UpdateIntent, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return UpdateIntent{Action: "none"}, nil
	}

	systemPrompt := buildUpdateIntentPrompt(ref)
	content, _, err := c.invokeLLM(ctx, "llm.ParseUpdateIntent", apiKey, systemPrompt, text)
	if err != nil {
		return UpdateIntent{}, err
	}
	if strings.TrimSpace(content) == "" {
		return UpdateIntent{Action: "none"}, nil
	}

	content = stripMarkdownFence(content)

	var intent UpdateIntent
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		tracing.RecordError(ctx, err)
		return UpdateIntent{}, fmt.Errorf("解析修改意图 JSON 失败: %w", err)
	}

	intent.Action = strings.TrimSpace(strings.ToLower(intent.Action))
	intent.Date = normalizeDate(intent.Date)
	intent.Title = strings.TrimSpace(intent.Title)
	intent.NewTitle = strings.TrimSpace(intent.NewTitle)
	intent.StartTime = normalizeTime(intent.StartTime)
	intent.EndTime = normalizeTime(intent.EndTime)
	if intent.Action != "update" {
		intent.Action = "none"
	}

	if b, mErr := json.Marshal(intent); mErr == nil {
		tracing.Event(ctx, "llm.update_intent_result", map[string]string{
			"intent": string(b),
		})
	}

	return intent, nil
}

func buildUpdateIntentPrompt(ref time.Time) string {
	weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
	today := ref.Format("2006-01-02")
	weekday := weekdays[int(ref.Weekday())]
	return fmt.Sprintf(`你是修改意图识别助手。请只输出 JSON 对象，不要 markdown，不要解释。

当前参考日期：%s（星期%s）

输出格式：
{
  "action": "update 或 none",
  "id": 0,
  "date": "YYYY-MM-DD 或 空字符串",
  "title": "目标日程标题或空字符串",
  "newTitle": "新标题或空字符串",
  "startTime": "HH:mm 或空字符串",
  "endTime": "HH:mm 或空字符串"
}

规则：
1) 若用户意图是修改日程，action=update，否则 action=none
2) 支持“今天/明天/后天/昨天/具体日期”等相对时间转 YYYY-MM-DD
3) title 是被修改的目标日程标识，newTitle 是修改后的标题
4) startTime/endTime 为修改后的时间，结束时间可为空字符串
5) 未提及 id 时 id=0；未提及字段用空字符串
6) 只输出 JSON 对象`, today, weekday)
}
