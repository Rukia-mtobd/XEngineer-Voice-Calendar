package calendar

import (
	"fmt"
	"time"

	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/db"
	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/parser"
)

type Service struct {
	db *db.Database
}

func New(database *db.Database) *Service {
	return &Service{db: database}
}

func (s *Service) CreateFromText(text string) (*db.Event, error) {
	parsed, err := parser.Parse(text, time.Now())
	if err != nil {
		return nil, err
	}
	id, err := s.db.CreateEvent(parsed.Title, parsed.ScheduledAt)
	if err != nil {
		return nil, err
	}
	return s.db.GetEvent(id)
}

func (s *Service) Create(title, scheduledAt string) (*db.Event, error) {
	t, err := time.Parse(time.RFC3339, scheduledAt)
	if err != nil {
		return nil, fmt.Errorf("时间格式无效，请使用 RFC3339")
	}
	id, err := s.db.CreateEvent(title, t)
	if err != nil {
		return nil, err
	}
	return s.db.GetEvent(id)
}

func (s *Service) Update(id int64, title, scheduledAt string) (*db.Event, error) {
	t, err := time.Parse(time.RFC3339, scheduledAt)
	if err != nil {
		return nil, fmt.Errorf("时间格式无效")
	}
	if err := s.db.UpdateEvent(id, title, t); err != nil {
		return nil, err
	}
	return s.db.GetEvent(id)
}

func (s *Service) Delete(id int64) error {
	return s.db.DeleteEvent(id)
}

func (s *Service) List() ([]db.Event, error) {
	return s.db.ListEvents()
}

func (s *Service) ListByDate(dateStr string) ([]db.Event, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("日期格式应为 YYYY-MM-DD")
	}
	return s.db.ListEventsByDate(t)
}
