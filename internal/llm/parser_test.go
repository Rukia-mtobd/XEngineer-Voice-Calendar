package llm

import (
	"testing"
	"time"
)

func TestNormalizeTime(t *testing.T) {
	tests := map[string]string{
		"09:30":   "09:30",
		" 18:05 ": "18:05",
		"24:00":   "",
		"09:60":   "",
		"9:30":    "",
	}
	for input, want := range tests {
		if got := normalizeTime(input); got != want {
			t.Errorf("normalizeTime(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestCoerceScheduleDate(t *testing.T) {
	ref := time.Date(2026, 8, 14, 12, 0, 0, 0, time.Local)
	if got := coerceScheduleDate("2024-08-20", ref); got != "2026-08-20" {
		t.Fatalf("coerceScheduleDate historical year = %q", got)
	}
	if got := coerceScheduleDate("2026-02-30", ref); got != "" {
		t.Fatalf("coerceScheduleDate invalid date = %q", got)
	}
}

func TestDecodeScheduleJSONArrayNormalizesLooseValues(t *testing.T) {
	ref := time.Date(2026, 8, 14, 12, 0, 0, 0, time.Local)
	items, err := decodeScheduleJSONArray(`[{"title":"开会","date":"2026-08-15","time":"09:00","duration":"30"}]`, "明天九点开会", ref)
	if err != nil {
		t.Fatalf("decodeScheduleJSONArray() error = %v", err)
	}
	if len(items) != 1 || items[0].StartTime != "09:00" || items[0].Duration != 30 || items[0].Desc == "" {
		t.Fatalf("decoded items = %+v", items)
	}
}
