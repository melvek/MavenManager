package services

import (
	"mvnmanager/internal/config"
)

type ConfigService struct{}

// GetConfig 提供给前端调用的接口
func (s *ConfigService) GetConfig() (*config.AppConfig, error) {
	return config.LoadConfig()
}

// SaveConfig 保存前端传来的配置
func (s *ConfigService) SaveConfig(cfg *config.AppConfig) error {
	return config.SaveConfig(cfg)
}

func (s *ConfigService) GetRepositoryInfos() (map[string]config.RepositoryInfo, error) {
	return config.GetRepositoryInfos()
}

func (s *ConfigService) SaveRepositoryInfo(id string, info config.RepositoryInfo) error {
	return config.SaveRepositoryInfo(id, info)
}

func (s *ConfigService) DeleteRepositoryInfo(id string) error {
	return config.DeleteRepositoryInfo(id)
}

func (s *ConfigService) NewRepositoryID() (string, error) {
	return config.NewRepositoryID()
}
