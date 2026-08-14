package reminder

import (
	"errors"
	"testing"
	"time"

	"xengineer-voice-calendar/internal/storage"
)

type fakeStore struct {
	rows []storage.ScheduleRecord
	err  error
}

func (f fakeStore) ListSchedules() ([]storage.ScheduleRecord, error) {
	return f.rows, f.err
}

type sentNotification struct {
	title string
	body  string
}

type fakeNotifier struct {
	sent []sentNotification
	err  error
}

func (f *fakeNotifier) Send(title, body string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, sentNotification{title: title, body: body})
	return nil
}

func TestCheckSendsUpcomingReminderOnce(t *testing.T) {
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, time.Local)
	store := fakeStore{rows: []storage.ScheduleRecord{
		{ID: 1, Date: "2026-08-14", StartTime: "09:05", Title: "项目会议", Desc: "会议室 A"},
		{ID: 2, Date: "2026-08-14", StartTime: "09:30", Title: "稍后日程"},
		{ID: 3, Date: "2026-08-14", StartTime: "08:59", Title: "已开始"},
	}}
	notifier := &fakeNotifier{}
	service := New(store, notifier, 10*time.Minute, time.Minute)

	if err := service.Check(now); err != nil {
		t.Fatalf("first Check() error = %v", err)
	}
	if err := service.Check(now.Add(time.Minute)); err != nil {
		t.Fatalf("second Check() error = %v", err)
	}

	if len(notifier.sent) != 1 {
		t.Fatalf("sent %d notifications, want 1", len(notifier.sent))
	}
	if got := notifier.sent[0].body; got != "09:05 · 项目会议\n会议室 A" {
		t.Fatalf("notification body = %q", got)
	}
}

func TestCheckRetriesAfterNotificationFailure(t *testing.T) {
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, time.Local)
	store := fakeStore{rows: []storage.ScheduleRecord{{ID: 1, Date: "2026-08-14", StartTime: "09:05", Title: "项目会议"}}}
	notifier := &fakeNotifier{err: errors.New("send failed")}
	service := New(store, notifier, 10*time.Minute, time.Minute)

	if err := service.Check(now); err == nil {
		t.Fatal("Check() error = nil, want notification error")
	}
	notifier.err = nil
	if err := service.Check(now); err != nil {
		t.Fatalf("retry Check() error = %v", err)
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("retry sent %d notifications, want 1", len(notifier.sent))
	}
}

func TestCheckReturnsStoreError(t *testing.T) {
	want := errors.New("database unavailable")
	service := New(fakeStore{err: want}, &fakeNotifier{}, 10*time.Minute, time.Minute)
	if err := service.Check(time.Now()); !errors.Is(err, want) {
		t.Fatalf("Check() error = %v, want %v", err, want)
	}
}
