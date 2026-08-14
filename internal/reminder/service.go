package reminder

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"xengineer-voice-calendar/internal/notification"
	"xengineer-voice-calendar/internal/storage"
)

const (
	DefaultLeadTime     = 10 * time.Minute
	DefaultPollInterval = time.Minute
)

type scheduleLister interface {
	ListSchedules() ([]storage.ScheduleRecord, error)
}

// Service checks upcoming schedules and emits each reminder once per run.
type Service struct {
	store        scheduleLister
	notifier     notification.Notifier
	leadTime     time.Duration
	pollInterval time.Duration

	mu       sync.Mutex
	notified map[string]time.Time
}

func New(store scheduleLister, notifier notification.Notifier, leadTime, pollInterval time.Duration) *Service {
	if leadTime <= 0 {
		leadTime = DefaultLeadTime
	}
	if pollInterval <= 0 {
		pollInterval = DefaultPollInterval
	}
	return &Service{
		store:        store,
		notifier:     notifier,
		leadTime:     leadTime,
		pollInterval: pollInterval,
		notified:     make(map[string]time.Time),
	}
}

// Run blocks until ctx is cancelled and checks once immediately at startup.
func (s *Service) Run(ctx context.Context) {
	_ = s.Check(time.Now())
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case now := <-ticker.C:
			_ = s.Check(now)
		case <-ctx.Done():
			return
		}
	}
}

// Check sends reminders scheduled in [now, now+leadTime].
func (s *Service) Check(now time.Time) error {
	if s.store == nil || s.notifier == nil {
		return nil
	}
	rows, err := s.store.ListSchedules()
	if err != nil {
		return err
	}

	cutoff := now.Add(s.leadTime)
	for _, row := range rows {
		start, ok := scheduleStart(row, now.Location())
		if !ok || start.Before(now) || start.After(cutoff) {
			continue
		}

		key := fmt.Sprintf("%d|%s|%s", row.ID, row.Date, row.StartTime)
		if !s.reserve(key, start, now) {
			continue
		}

		body := fmt.Sprintf("%s · %s", row.StartTime, strings.TrimSpace(row.Title))
		if desc := strings.TrimSpace(row.Desc); desc != "" && desc != strings.TrimSpace(row.Title) {
			body += "\n" + desc
		}
		if err := s.notifier.Send("日程即将开始", body); err != nil {
			s.release(key)
			return err
		}
	}
	return nil
}

func (s *Service) reserve(key string, start, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, at := range s.notified {
		if at.Before(now.Add(-24 * time.Hour)) {
			delete(s.notified, k)
		}
	}
	if _, exists := s.notified[key]; exists {
		return false
	}
	s.notified[key] = start
	return true
}

func (s *Service) release(key string) {
	s.mu.Lock()
	delete(s.notified, key)
	s.mu.Unlock()
}

func scheduleStart(row storage.ScheduleRecord, loc *time.Location) (time.Time, bool) {
	if strings.TrimSpace(row.Date) == "" || strings.TrimSpace(row.StartTime) == "" {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", row.Date+" "+row.StartTime, loc)
	return t, err == nil
}
