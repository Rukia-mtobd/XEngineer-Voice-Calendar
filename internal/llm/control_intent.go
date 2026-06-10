package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"xengineer-voice-calendar/internal/tracing"
)

// ControlIntent 是界面控制类语音意图识别结果。
// action 仅为 control 或 none。
type ControlIntent struct {
	Action  string `json:"action"`
	Command string `json:"command"`
}

func (c *Client) ParseControlIntent(ctx context.Context, apiKey, text string, ref time.Time) (ControlIntent, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return ControlIntent{Action: "none"}, nil
	}

	systemPrompt := buildControlIntentPrompt(ref)
	content, _, err := c.invokeLLM(ctx, "llm.ParseControlIntent", apiKey, systemPrompt, text)
	if err != nil {
		return ControlIntent{}, err
	}
	if strings.TrimSpace(content) == "" {
		return ControlIntent{Action: "none"}, nil
	}

	content = stripMarkdownFence(content)

	var intent ControlIntent
	if err := json.Unmarshal([]byte(content), &intent); err != nil {
		tracing.RecordError(ctx, err)
		return ControlIntent{}, fmt.Errorf("解析控制意图 JSON 失败: %w", err)
	}

	intent.Action = strings.TrimSpace(strings.ToLower(intent.Action))
	intent.Command = strings.TrimSpace(strings.ToLower(intent.Command))
	if intent.Action != "control" {
		intent.Action = "none"
	}

	switch intent.Command {
	case "view_all", "close_all", "view_todo", "view_done", "view_today", "open_settings", "close_settings", "open_important", "close_important", "theme_dark", "theme_light", "go_home":
		if b, mErr := json.Marshal(intent); mErr == nil {
			tracing.Event(ctx, "llm.control_intent_result", map[string]string{
				"intent": string(b),
			})
		}
		return intent, nil
	default:
		tracing.Event(ctx, "llm.control_intent_rejected", map[string]string{
			"command": intent.Command,
		})
		return ControlIntent{Action: "none"}, nil
	}
}

func buildControlIntentPrompt(ref time.Time) string {
	weekdays := []string{"日", "一", "二", "三", "四", "五", "六"}
	today := ref.Format("2006-01-02")
	weekday := weekdays[int(ref.Weekday())]
	return fmt.Sprintf(`你是界面控制意图识别助手。请只输出 JSON 对象，不要 markdown，不要解释。

当前参考日期：%s（星期%s）

输出格式：
{
  "action": "control 或 none",
  "command": "view_all | close_all | view_todo | view_done | view_today | open_settings | close_settings | open_important | close_important | theme_dark | theme_light | go_home 或 空字符串"
}

规则：
1) 仅当用户意图是控制软件界面/功能时，action=control；否则 action=none
2) 指令映射：
   - “查看全部日程” => view_all
   - “关闭全部日程/关闭所有日程” => close_all
   - “查看待完成日程” => view_todo
   - “查看已完成日程” => view_done
   - “查看今天日程” => view_today
   - “打开设置” => open_settings
   - “关闭设置” => close_settings
   - “打开重要日程/查看重要日程” => open_important
   - “关闭重要日程” => close_important
   - “切换黑色模式/切换深色模式” => theme_dark
   - “切换浅色模式” => theme_light
   - “回到主页” => go_home
3) 不要把“新增、删除、修改日程”识别为 control，应输出 action=none
4) 只输出 JSON 对象`, today, weekday)
}
