package storage

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const defaultDBFile = "voice_calendar.db"

// ScheduleRecord 是日程持久化模型。
type ScheduleRecord struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Date        string    `json:"date" gorm:"index;size:10;not null"` // YYYY-MM-DD
	Title       string    `json:"title" gorm:"size:255;not null"`
	StartTime   string    `json:"startTime" gorm:"size:8"`
	EndTime     string    `json:"endTime" gorm:"size:8"`
	Duration    int       `json:"duration"`
	Desc        string    `json:"desc" gorm:"type:text"`
	IsImportant bool      `json:"isImportant" gorm:"not null;default:false;index"`
	CreatedAt   time.Time `json:"createdAt"`
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

// ListImportantSchedules 返回全部被标记为重要的日程。
func (s *Store) ListImportantSchedules() ([]ScheduleRecord, error) {
	var rows []ScheduleRecord
	if err := s.db.
		Where("is_important = ?", true).
		Order("date asc").
		Order("start_time asc").
		Order("created_at desc").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// ListScheduledDatesByMonth 返回指定月份（YYYY-MM）内有日程的去重日期列表（YYYY-MM-DD）。
func (s *Store) ListScheduledDatesByMonth(month string) ([]string, error) {
	month = strings.TrimSpace(month)
	var dates []string
	if err := s.db.
		Model(&ScheduleRecord{}).
		Distinct("date").
		Where("date LIKE ?", month+"-%").
		Order("date asc").
		Pluck("date", &dates).Error; err != nil {
		return nil, err
	}
	return dates, nil
}

func (s *Store) DeleteScheduleByID(id uint) error {
	return s.db.Delete(&ScheduleRecord{}, id).Error
}

func (s *Store) DeleteScheduleByDateAndTitle(date, title string) (int64, error) {
	res := s.db.Where("date = ? AND title = ?", strings.TrimSpace(date), strings.TrimSpace(title)).Delete(&ScheduleRecord{})
	return res.RowsAffected, res.Error
}

func (s *Store) DeleteScheduleByDateAndStartTime(date, startTime string) (int64, error) {
	res := s.db.Where("date = ? AND start_time = ?", strings.TrimSpace(date), strings.TrimSpace(startTime)).Delete(&ScheduleRecord{})
	return res.RowsAffected, res.Error
}

func (s *Store) DeleteScheduleByDateTitleAndStartTime(date, title, startTime string) (int64, error) {
	res := s.db.Where(
		"date = ? AND title = ? AND start_time = ?",
		strings.TrimSpace(date),
		strings.TrimSpace(title),
		strings.TrimSpace(startTime),
	).Delete(&ScheduleRecord{})
	return res.RowsAffected, res.Error
}

func (s *Store) DeleteSchedulesByDate(date string) (int64, error) {
	res := s.db.Where("date = ?", strings.TrimSpace(date)).Delete(&ScheduleRecord{})
	return res.RowsAffected, res.Error
}

func (s *Store) UpdateScheduleByID(id uint, title, startTime, endTime string) (ScheduleRecord, error) {
	title = strings.TrimSpace(title)
	startTime = strings.TrimSpace(startTime)
	endTime = strings.TrimSpace(endTime)

	updates := map[string]any{
		"title":      title,
		"start_time": startTime,
		"end_time":   endTime,
	}
	if startTime != "" && endTime != "" {
		updates["duration"] = calcDurationMinutes(startTime, endTime)
	} else if endTime == "" {
		updates["duration"] = 0
	}

	if err := s.db.Model(&ScheduleRecord{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return ScheduleRecord{}, err
	}

	var row ScheduleRecord
	if err := s.db.First(&row, id).Error; err != nil {
		return ScheduleRecord{}, err
	}
	return row, nil
}

// UpdateScheduleByIDWithDesc 按 ID 更新标题、开始时间、结束时间和备注（结束时间可空）。
func (s *Store) UpdateScheduleByIDWithDesc(id uint, title, startTime, endTime, desc string) (ScheduleRecord, error) {
	title = strings.TrimSpace(title)
	startTime = strings.TrimSpace(startTime)
	endTime = strings.TrimSpace(endTime)
	desc = strings.TrimSpace(desc)

	updates := map[string]any{
		"title":      title,
		"start_time": startTime,
		"end_time":   endTime,
		"desc":       desc,
	}
	if startTime != "" && endTime != "" {
		updates["duration"] = calcDurationMinutes(startTime, endTime)
	} else if endTime == "" {
		updates["duration"] = 0
	}

	if err := s.db.Model(&ScheduleRecord{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return ScheduleRecord{}, err
	}

	var row ScheduleRecord
	if err := s.db.First(&row, id).Error; err != nil {
		return ScheduleRecord{}, err
	}
	return row, nil
}

// SetScheduleImportant 按 ID 设置日程是否重要。
func (s *Store) SetScheduleImportant(id uint, important bool) (ScheduleRecord, error) {
	if err := s.db.Model(&ScheduleRecord{}).
		Where("id = ?", id).
		Update("is_important", important).Error; err != nil {
		return ScheduleRecord{}, err
	}
	var row ScheduleRecord
	if err := s.db.First(&row, id).Error; err != nil {
		return ScheduleRecord{}, err
	}
	return row, nil
}

func calcDurationMinutes(start, end string) int {
	startT, err1 := time.Parse("15:04", start)
	endT, err2 := time.Parse("15:04", end)
	if err1 != nil || err2 != nil {
		return 0
	}
	minutes := int(endT.Sub(startT).Minutes())
	if minutes < 0 {
		minutes += 24 * 60
	}
	return minutes
}
