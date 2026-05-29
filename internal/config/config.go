package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type fileConfig struct {
	APIKey string `json:"api_key"`
}

func configPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "config.json"), nil
}

// SaveApiKey 将 API Key 写入程序目录下的 config.json。
func SaveApiKey(key string) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(fileConfig{APIKey: key}, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadApiKey 从程序目录下的 config.json 读取 API Key。
func LoadApiKey() (string, error) {
	path, err := configPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	var cfg fileConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", err
	}

	return cfg.APIKey, nil
}
