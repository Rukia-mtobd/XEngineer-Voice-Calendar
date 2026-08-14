package storage

import (
	"path/filepath"
	"testing"
)

func TestStoreScheduleLifecycle(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "calendar.db"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	created, err := store.CreateSchedule(ScheduleRecord{
		Date: "2026-08-14", Title: "夜间维护", StartTime: "23:30", EndTime: "00:30",
	})
	if err != nil {
		t.Fatalf("CreateSchedule() error = %v", err)
	}

	updated, err := store.UpdateScheduleByIDWithDesc(created.ID, "夜间维护", "23:30", "00:30", "生产环境")
	if err != nil {
		t.Fatalf("UpdateScheduleByIDWithDesc() error = %v", err)
	}
	if updated.Duration != 60 || updated.Desc != "生产环境" {
		t.Fatalf("updated record = %+v", updated)
	}

	if _, err := store.SetScheduleImportant(created.ID, true); err != nil {
		t.Fatalf("SetScheduleImportant() error = %v", err)
	}
	important, err := store.ListImportantSchedules()
	if err != nil || len(important) != 1 {
		t.Fatalf("ListImportantSchedules() = %v, %v", important, err)
	}

	dates, err := store.ListScheduledDatesByMonth("2026-08")
	if err != nil || len(dates) != 1 || dates[0] != "2026-08-14" {
		t.Fatalf("ListScheduledDatesByMonth() = %v, %v", dates, err)
	}

	if err := store.DeleteScheduleByID(created.ID); err != nil {
		t.Fatalf("DeleteScheduleByID() error = %v", err)
	}
	rows, err := store.ListSchedules()
	if err != nil || len(rows) != 0 {
		t.Fatalf("ListSchedules() after delete = %v, %v", rows, err)
	}
}

func TestCalcDurationMinutes(t *testing.T) {
	tests := []struct {
		name       string
		start, end string
		want       int
	}{
		{name: "same day", start: "09:15", end: "10:45", want: 90},
		{name: "cross midnight", start: "23:30", end: "00:15", want: 45},
		{name: "invalid", start: "99:00", end: "10:00", want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calcDurationMinutes(tt.start, tt.end); got != tt.want {
				t.Fatalf("calcDurationMinutes(%q, %q) = %d, want %d", tt.start, tt.end, got, tt.want)
			}
		})
	}
}
