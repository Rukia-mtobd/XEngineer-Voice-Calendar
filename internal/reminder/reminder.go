package reminder

import (
	"context"
	"time"

	"github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/db"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Service struct {
	db       *db.Database
	interval time.Duration
}

func New(database *db.Database) *Service {
	return &Service{
		db:       database,
		interval: 15 * time.Second,
	}
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkDue(ctx)
		}
	}
}

func (s *Service) checkDue(ctx context.Context) {
	events, err := s.db.ListDueEvents(time.Now())
	if err != nil {
		return
	}
	for _, event := range events {
		runtime.EventsEmit(ctx, "reminder", event)
		_ = s.db.MarkReminded(event.ID)
	}
}
