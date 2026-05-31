package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	reTimeHM     = regexp.MustCompile(`(\d{1,2})\s*[:：点时]\s*(\d{1,2})?`)
	reTimeHour   = regexp.MustCompile(`(\d{1,2})\s*点`)
	reDateMD     = regexp.MustCompile(`(\d{1,2})\s*月\s*(\d{1,2})\s*日?`)
	reWeekday    = regexp.MustCompile(`(?:周|星期)([一二三四五六日天])`)
	reRelative   = regexp.MustCompile(`(今天|今日|明天|明日|后天|大后天)`)
	reAfterDays  = regexp.MustCompile(`(\d+)\s*天[之后内]`)
	removeWords  = regexp.MustCompile(`(提醒|帮我|请|安排|创建|添加|新增|设置|一下|记得|别忘了|日程|日历|在|于|到)`)
)

var weekdayMap = map[string]time.Weekday{
	"一": time.Monday,
	"二": time.Tuesday,
	"三": time.Wednesday,
	"四": time.Thursday,
	"五": time.Friday,
	"六": time.Saturday,
	"日": time.Sunday,
	"天": time.Sunday,
}

type ParsedEvent struct {
	Title       string
	ScheduledAt time.Time
}

// Parse 从中文语音文本中提取日程标题与时间。
func Parse(text string, now time.Time) (*ParsedEvent, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("文本为空")
	}

	scheduledAt, rest := extractDateTime(text, now)
	title := cleanTitle(rest)
	if title == "" {
		title = cleanTitle(text)
	}
	if title == "" {
		title = "未命名日程"
	}
	if scheduledAt.IsZero() {
		scheduledAt = defaultTime(now)
	}

	return &ParsedEvent{
		Title:       title,
		ScheduledAt: scheduledAt,
	}, nil
}

func extractDateTime(text string, now time.Time) (time.Time, string) {
	loc := now.Location()
	base := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, loc)
	rest := text

	if m := reRelative.FindStringSubmatch(text); len(m) > 1 {
		switch m[1] {
		case "今天", "今日":
			base = time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, loc)
		case "明天", "明日":
			t := now.AddDate(0, 0, 1)
			base = time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, loc)
		case "后天":
			t := now.AddDate(0, 0, 2)
			base = time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, loc)
		case "大后天":
			t := now.AddDate(0, 0, 3)
			base = time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, loc)
		}
		rest = strings.Replace(rest, m[0], "", 1)
	}

	if m := reAfterDays.FindStringSubmatch(text); len(m) > 1 {
		if days, err := strconv.Atoi(m[1]); err == nil {
			t := now.AddDate(0, 0, days)
			base = time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, loc)
			rest = strings.Replace(rest, m[0], "", 1)
		}
	}

	if m := reDateMD.FindStringSubmatch(text); len(m) > 2 {
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])
		year := now.Year()
		candidate := time.Date(year, time.Month(month), day, 9, 0, 0, 0, loc)
		if candidate.Before(now.AddDate(0, 0, -1)) {
			candidate = candidate.AddDate(1, 0, 0)
		}
		base = candidate
		rest = strings.Replace(rest, m[0], "", 1)
	}

	if m := reWeekday.FindStringSubmatch(text); len(m) > 1 {
		if wd, ok := weekdayMap[m[1]]; ok {
			base = nextWeekday(now, wd)
			rest = strings.Replace(rest, m[0], "", 1)
		}
	}

	hour, minute, isPM, hasTime := parseClock(text)
	if hasTime {
		if isPM && hour < 12 {
			hour += 12
		}
		if !isPM && strings.Contains(text, "下午") && hour < 12 {
			hour += 12
		}
		if strings.Contains(text, "中午") && hour <= 12 {
			hour = 12
		}
		if strings.Contains(text, "晚上") || strings.Contains(text, "晚间") {
			if hour < 12 {
				hour += 12
			}
		}
		if strings.Contains(text, "凌晨") && hour == 12 {
			hour = 0
		}
		base = time.Date(base.Year(), base.Month(), base.Day(), hour, minute, 0, 0, loc)
	}

	return base, rest
}

func parseClock(text string) (hour, minute int, isPM bool, ok bool) {
	if strings.Contains(text, "下午") || strings.Contains(text, "晚上") || strings.Contains(text, "晚间") {
		isPM = true
	}

	if m := reTimeHM.FindStringSubmatch(text); len(m) > 1 {
		hour, _ = strconv.Atoi(m[1])
		if len(m) > 2 && m[2] != "" {
			minute, _ = strconv.Atoi(m[2])
		}
		if strings.Contains(text, "半") && minute == 0 {
			minute = 30
		}
		return hour, minute, isPM, true
	}

	if m := reTimeHour.FindStringSubmatch(text); len(m) > 1 {
		hour, _ = strconv.Atoi(m[1])
		if strings.Contains(text, "半") {
			minute = 30
		}
		return hour, minute, isPM, true
	}
	return 0, 0, false, false
}

func nextWeekday(now time.Time, target time.Weekday) time.Time {
	loc := now.Location()
	days := (int(target) - int(now.Weekday()) + 7) % 7
	if days == 0 {
		days = 7
	}
	t := now.AddDate(0, 0, days)
	return time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, loc)
}

func defaultTime(now time.Time) time.Time {
	loc := now.Location()
	t := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, loc)
	if t.Before(now) {
		t = t.Add(time.Hour)
	}
	if t.Hour() < 8 {
		t = time.Date(t.Year(), t.Month(), t.Day(), 9, 0, 0, 0, loc)
	}
	return t
}

func cleanTitle(text string) string {
	text = reRelative.ReplaceAllString(text, "")
	text = reAfterDays.ReplaceAllString(text, "")
	text = reDateMD.ReplaceAllString(text, "")
	text = reWeekday.ReplaceAllString(text, "")
	text = reTimeHM.ReplaceAllString(text, "")
	text = reTimeHour.ReplaceAllString(text, "")
	text = removeWords.ReplaceAllString(text, "")
	text = strings.NewReplacer(
		"上午", "", "下午", "", "晚上", "", "中午", "", "凌晨", "",
		"明日", "", "今日", "", "今天", "", "明天", "", "后天", "", "大后天", "",
		"点半", "", "点", "", "分", "", "时", "",
	).Replace(text)
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.Trim(text, "，,。.、 ")
}
