package parser

import (
	"testing"
	"time"
)

func TestParseTomorrowAfternoon(t *testing.T) {
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.Local)
	parsed, err := Parse("明天下午3点开会", now)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Title == "" {
		t.Fatal("expected title")
	}
	if parsed.ScheduledAt.Day() != 30 {
		t.Fatalf("expected day 30, got %d", parsed.ScheduledAt.Day())
	}
	if parsed.ScheduledAt.Hour() != 15 {
		t.Fatalf("expected hour 15, got %d", parsed.ScheduledAt.Hour())
	}
}

func TestParseWeekday(t *testing.T) {
	now := time.Date(2026, 5, 29, 10, 0, 0, 0, time.Local) // Thursday
	parsed, err := Parse("周五上午10点体检", now)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ScheduledAt.Weekday() != time.Friday {
		t.Fatalf("expected Friday, got %v", parsed.ScheduledAt.Weekday())
	}
}
