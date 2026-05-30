package storage

import (
	"fmt"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const defaultDBFile = "voice_calendar.db"

// ScheduleRecord 是日程持久化模型。
type ScheduleRecord struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Date       string    `json:"date" gorm:"index;size:10;not null"` // YYYY-MM-DD
	Title      string    `json:"title" gorm:"size:255;not null"`
	StartTime  string    `json:"startTime" gorm:"size:8"`
	EndTime    string    `json:"endTime" gorm:"size:8"`
	Duration   int       `json:"duration"`
	Desc       string    `json:"desc" gorm:"type:text"`
	CreatedAt  time.Time `json:"createdAt"`
}

func (ScheduleRecord) TableName() string {
	return "schedules"
}

type Store struct {
	db *gorm.DB
}

// NewStore 初始化 SQLite 与表结构。
// dbPath 为空时，默认使用项目目录下 voice_calendar.db。
func NewStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		dbPath = defaultDBFile
	}
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("resolve db path: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(absPath), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := db.AutoMigrate(&ScheduleRecord{}); err != nil {
		return nil, fmt.Errorf("auto migrate schedules: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) CreateSchedule(item ScheduleRecord) (ScheduleRecord, error) {
	if err := s.db.Create(&item).Error; err != nil {
		return ScheduleRecord{}, err
	}
	return item, nil
}

func (s *Store) ListSchedules() ([]ScheduleRecord, error) {
	var rows []ScheduleRecord
	if err := s.db.
		Order("date asc").
		Order("start_time asc").
		Order("created_at desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) ListSchedulesByDate(date string) ([]ScheduleRecord, error) {
	var rows []ScheduleRecord
	if err := s.db.
		Where("date = ?", date).
		Order("start_time asc").
		Order("created_at desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
