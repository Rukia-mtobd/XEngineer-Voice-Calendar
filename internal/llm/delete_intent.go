package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"xengineer-voice-calendar/internal/tracing"
)

// DeleteIntent 是删除语义识别结果。
// action 仅为 delete 或 none。
type DeleteIntent struct {
	Action    string `json:"action"`
	ID        uint   `json:"id"`
	Date      string `json:"date"`
	Title     string `json:"title"`
	StartTime string `json:"startTime"`
}

func (c *Client) ParseDeleteIntent(ctx context.Context, apiKey, text string, ref time.Time) (DeleteIntent, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return DeleteIntent{Action: "none"}, nil
	}
	if shouldIgnoreAsCreateForDelete(text) || looksLikeScheduleCreate(text) {
		reason := "create_keyword_guard"
		if looksLikeScheduleCreate(text) && !shouldIgnoreAsCreateForDelete(text) {
			reason = "schedule_create_pattern_guard"
		}
		tracing.Event(ctx, "llm.delete_skipped", map[string]string{
			"reason": reason,
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
	intent.StartTime = normalizeTime(intent.StartTime)
	if intent.Action != "delete" {
		intent.Action = "none"
	}
	intent = enrichDeleteIntent(ctx, intent, text)
	if intent.Action == "delete" && !hasExplicitDeleteKeyword(text) {
		tracing.Event(ctx, "llm.delete_downgraded", map[string]string{
			"reason": "no_explicit_delete_keyword",
			"text":   text,
		})
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
	normalized := normalizeIntentText(text)
	if normalized == "" {
		return false
	}

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

// hasExplicitDeleteKeyword 要求用户原文含明确删除语义，防止「明天下午三点跑步」类新建句被 LLM 误判为删除。
func hasExplicitDeleteKeyword(text string) bool {
	normalized := normalizeIntentText(text)
	if normalized == "" {
		return false
	}
	for _, kw := range []string{
		"删除", "删掉", "删了", "移除", "去掉", "取消", "清空", "不要了", "干掉", "清除",
		"delete", "remove", "cancel", "clear",
	} {
		if strings.Contains(normalized, kw) {
			return true
		}
	}
	return false
}

// looksLikeScheduleCreate 检测「日期/时刻 + 事件」类新建表述，无删除词时跳过删除意图 LLM。
func looksLikeScheduleCreate(text string) bool {
	if hasExplicitDeleteKeyword(text) {
		return false
	}
	normalized := normalizeIntentText(text)
	if normalized == "" {
		return false
	}
	for _, kw := range []string{
		"改到", "改成", "改为", "改在", "推迟", "提前", "换到", "移到", "调整",
		"update", "reschedule", "move to",
	} {
		if strings.Contains(normalized, kw) {
			return false
		}
	}
	hasDate := false
	for _, kw := range []string{
		"今天", "明天", "后天", "大后天", "昨天", "前天",
		"下周", "本周", "这周",
		"周一", "周二", "周三", "周四", "周五", "周六", "周日",
		"星期一", "星期二", "星期三", "星期四", "星期五", "星期六", "星期日",
		"元旦", "春节",
	} {
		if strings.Contains(normalized, kw) {
			hasDate = true
			break
		}
	}
	hasTime := false
	for _, kw := range []string{
		"点", "点半", "上午", "下午", "晚上", "中午", "凌晨", "清晨", "傍晚",
		"am", "pm",
	} {
		if strings.Contains(normalized, kw) {
			hasTime = true
			break
		}
	}
	if !hasTime {
		for i := 0; i < len(normalized)-1; i++ {
			if normalized[i] == ':' && normalized[i+1] >= '0' && normalized[i+1] <= '9' {
				hasTime = true
				break
			}
		}
	}
	return hasDate && hasTime
}

func normalizeIntentText(text string) string {
	normalized := strings.ToLower(strings.TrimSpace(text))
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
}

func enrichDeleteIntent(ctx context.Context, intent DeleteIntent, text string) DeleteIntent {
	if intent.Action != "delete" {
		return intent
	}
	if intent.StartTime == "" {
		if t := extractStartTimeFromText(text); t != "" {
			intent.StartTime = t
			tracing.Event(ctx, "llm.delete_time_extracted", map[string]string{
				"start_time": t,
				"text":       text,
			})
		}
	}
	// 仅有日期、无标题/时刻，且用户未说「全部/所有」→ 不应整 day 删除，降级为 none
	if intent.Date != "" && intent.Title == "" && intent.StartTime == "" && !wantsDeleteAllSchedules(text) {
		tracing.Event(ctx, "llm.delete_downgraded", map[string]string{
			"reason": "date_only_without_delete_all",
			"text":   text,
		})
		intent.Action = "none"
	}
	return intent
}

func wantsDeleteAllSchedules(text string) bool {
	n := normalizeIntentText(text)
	for _, kw := range []string{"全部", "所有", "全删", "清空", "all schedules", "all"} {
		if strings.Contains(n, kw) {
			return true
		}
	}
	return false
}

var cnHourDigits = map[rune]int{
	'零': 0, '一': 1, '二': 2, '两': 2, '三': 3, '四': 4,
	'五': 5, '六': 6, '七': 7, '八': 8, '九': 9, '十': 10,
}

// extractStartTimeFromText 从口语中提取时刻，供 LLM 未填 startTime 时兜底。
func extractStartTimeFromText(text string) string {
	normalized := strings.TrimSpace(text)
	if normalized == "" {
		return ""
	}
	if m := regexp.MustCompile(`(\d{1,2}):(\d{2})`).FindStringSubmatch(normalized); len(m) == 3 {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if h >= 0 && h <= 23 && min >= 0 && min <= 59 {
			return fmt.Sprintf("%02d:%02d", h, min)
		}
	}
	re := regexp.MustCompile(`(上午|下午|晚上|中午|凌晨)?\s*([零一二两三四五六七八九十\d]{1,3})\s*点(半)?`)
	m := re.FindStringSubmatch(normalized)
	if len(m) < 3 {
		return ""
	}
	hour := parseCNHourToken(m[2])
	if hour < 0 {
		return ""
	}
	minute := 0
	if len(m) > 4 && m[4] == "半" {
		minute = 30
	}
	period := m[1]
	switch period {
	case "下午", "晚上":
		if hour >= 1 && hour <= 11 {
			hour += 12
		}
	case "中午":
		if hour >= 1 && hour <= 3 {
			hour += 12
		}
	case "凌晨":
		if hour == 12 {
			hour = 0
		}
	case "上午", "":
		if hour == 12 {
			hour = 0
		}
	}
	if hour < 0 || hour > 23 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", hour, minute)
}

func parseCNHourToken(token string) int {
	token = strings.TrimSpace(token)
	if token == "" {
		return -1
	}
	if n, err := strconv.Atoi(token); err == nil {
		return n
	}
	if strings.Contains(token, "十") {
		if token == "十" {
			return 10
		}
		parts := strings.Split(token, "十")
		tens := 1
		if parts[0] != "" {
			if v, ok := cnHourDigits[rune(parts[0][0])]; ok {
				tens = v
			} else {
				return -1
			}
		}
		ones := 0
		if len(parts) > 1 && parts[1] != "" {
			if v, ok := cnHourDigits[rune(parts[1][0])]; ok {
				ones = v
			} else {
				return -1
			}
		}
		return tens*10 + ones
	}
	if len([]rune(token)) == 1 {
		if v, ok := cnHourDigits[rune(token[0])]; ok {
			return v
		}
	}
	return -1
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
  "title": "标题或空字符串",
  "startTime": "HH:mm 或 空字符串"
}

规则：
1) 仅当用户明确表达删除/取消/移除日程时，action=delete；否则 action=none
2) 「明天下午三点跑步」「后天上午开会」等只有日期+时刻+事件、没有删除词的句子，action 必须为 none（这是新建日程，不是删除）
3) 若用户给出“今天/明天/昨天/后天/具体日期”等，请转换成 YYYY-MM-DD
4) 仅当用户明确说“删除今天所有/全部日程”时，才 date=当天且 title="" 且 startTime=""
5) 「删除明天下午五点的日程/吃饭」→ action=delete，date=对应日期，startTime=17:00；title 填事件名（如吃饭），无事件名可留空
6) 「删除明天下午三点的项目会」→ action=delete，date=对应日期，title=项目会，startTime=15:00
7) 用户指定了具体时刻时，必须填写 startTime；禁止把「删除某日某时刻」理解成删除该日全部日程
8) 未提及 id 时 id=0；无法识别日期时 date=""
9) 只输出 JSON 对象`, today, weekday)
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
