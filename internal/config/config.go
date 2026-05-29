package config

import "github.com/Rukia-mtobd/XEngineer-Voice-Calendar/internal/db"

const APIKeyConfigKey = "dashscope_api_key"

type Service struct {
	db *db.Database
}

func New(database *db.Database) *Service {
	return &Service{db: database}
}

func (s *Service) GetAPIKey() (string, error) {
	return s.db.GetConfig(APIKeyConfigKey)
}

func (s *Service) SaveAPIKey(key string) error {
	return s.db.SetConfig(APIKeyConfigKey, key)
}
