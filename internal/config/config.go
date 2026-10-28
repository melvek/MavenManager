package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mvnmanager/internal/catalog"
)

type Repository struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type RepositoryInfo struct {
	Overview      *catalog.RepositoryOverview `json:"overview,omitempty"`
	LastScannedAt string                      `json:"lastScannedAt,omitempty"`
	LastCleanupAt string                      `json:"lastCleanupAt,omitempty"`
}

type RemoteRepository struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type AppConfig struct {
	MavenPath          string             `json:"mavenPath"`
	Repositories       []Repository       `json:"repositories"`
	ActiveRepoID       string             `json:"activeRepoId"`
	RemoteRepositories []RemoteRepository `json:"remoteRepositories,omitempty"`
}

// getConfigPath 获取配置文件路径 (与 exe 同级)
func getConfigPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}

	exeDir := filepath.Dir(exePath)

	// 开发模式判断：Wails 开发时，exe 会在系统临时目录 (Temp/go-build...)
	// 此时我们将配置放到项目的当前运行目录下，方便开发调试
	lowerDir := strings.ToLower(exeDir)
	if strings.Contains(lowerDir, "temp") || strings.Contains(lowerDir, "go-build") {
		wd, err := os.Getwd()
		if err == nil {
			exeDir = wd
		}
	}

	return filepath.Join(exeDir, "config.json"), nil
}

func repositoryInfoPath() (string, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(configPath), "repository-info.json"), nil
}

func NewRepositoryID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("generate repository ID: %w", err)
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(id[:])
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:]), nil
}

func GetRepositoryInfos() (map[string]RepositoryInfo, error) {
	path, err := repositoryInfoPath()
	if err != nil {
		return nil, err
	}
	return readRepositoryInfos(path)
}

func SaveRepositoryInfo(id string, info RepositoryInfo) error {
	path, err := repositoryInfoPath()
	if err != nil {
		return err
	}
	infos, err := readRepositoryInfos(path)
	if err != nil {
		return err
	}
	infos[id] = info
	return writeRepositoryInfos(path, infos)
}

func DeleteRepositoryInfo(id string) error {
	path, err := repositoryInfoPath()
	if err != nil {
		return err
	}
	infos, err := readRepositoryInfos(path)
	if err != nil {
		return err
	}
	delete(infos, id)
	return writeRepositoryInfos(path, infos)
}

func readRepositoryInfos(path string) (map[string]RepositoryInfo, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]RepositoryInfo{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read repository info: %w", err)
	}
	var infos map[string]RepositoryInfo
	if err := json.Unmarshal(data, &infos); err != nil {
		return nil, fmt.Errorf("decode repository info: %w", err)
	}
	if infos == nil {
		infos = map[string]RepositoryInfo{}
	}
	return infos, nil
}

func writeRepositoryInfos(path string, infos map[string]RepositoryInfo) error {
	data, err := json.MarshalIndent(infos, "", "  ")
	if err != nil {
		return fmt.Errorf("encode repository info: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write repository info: %w", err)
	}
	return nil
}

func pruneRepositoryInfos(path string, repositories []Repository) error {
	infos, err := readRepositoryInfos(path)
	if err != nil {
		return err
	}
	allowed := make(map[string]struct{}, len(repositories))
	for _, repo := range repositories {
		allowed[repo.ID] = struct{}{}
	}
	for id := range infos {
		if _, exists := allowed[id]; !exists {
			delete(infos, id)
		}
	}
	return writeRepositoryInfos(path, infos)
}

// LoadConfig 读取配置，如果不存在则初始化默认配置
func LoadConfig() (*AppConfig, error) {
	path, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// 如果没有配置文件，生成默认的 (获取默认 .m2 路径)
		home, _ := os.UserHomeDir()
		defaultM2 := filepath.Join(home, ".m2", "repository")

		defaultID, err := NewRepositoryID()
		if err != nil {
			return nil, err
		}
		defaultConfig := &AppConfig{
			MavenPath: "mvn", // 默认使用系统环境变量里的 mvn
			Repositories: []Repository{
				{ID: defaultID, Name: "默认仓库", Path: defaultM2},
			},
			ActiveRepoID: defaultID,
		}
		if err := SaveConfig(defaultConfig); err != nil {
			return nil, err
		}
		return defaultConfig, nil
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	var legacy struct {
		Repositories []struct {
			ID            string                      `json:"id"`
			Overview      *catalog.RepositoryOverview `json:"overview"`
			LastScannedAt string                      `json:"lastScannedAt"`
			LastCleanupAt string                      `json:"lastCleanupAt"`
		} `json:"repositories"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, err
	}

	infos, err := GetRepositoryInfos()
	if err != nil {
		return nil, err
	}
	oldToNewID := make(map[string]string)
	changedIDs := false
	hasLegacyInfo := false
	for i := range cfg.Repositories {
		oldID := cfg.Repositories[i].ID
		if !isUUID(oldID) {
			newID, err := NewRepositoryID()
			if err != nil {
				return nil, err
			}
			cfg.Repositories[i].ID = newID
			oldToNewID[oldID] = newID
			if cfg.ActiveRepoID == oldID {
				cfg.ActiveRepoID = newID
			}
			changedIDs = true
		}
	}
	for _, repo := range legacy.Repositories {
		if repo.Overview == nil && repo.LastScannedAt == "" && repo.LastCleanupAt == "" {
			continue
		}
		hasLegacyInfo = true
		id := repo.ID
		if newID, exists := oldToNewID[id]; exists {
			id = newID
		}
		if _, exists := infos[id]; !exists {
			infos[id] = RepositoryInfo{
				Overview:      repo.Overview,
				LastScannedAt: repo.LastScannedAt,
				LastCleanupAt: repo.LastCleanupAt,
			}
		}
	}
	for oldID, newID := range oldToNewID {
		if oldInfo, exists := infos[oldID]; exists {
			if _, migrated := infos[newID]; !migrated {
				infos[newID] = oldInfo
			}
			delete(infos, oldID)
		}
	}
	if changedIDs || hasLegacyInfo || len(infos) > 0 {
		infoPath, err := repositoryInfoPath()
		if err != nil {
			return nil, err
		}
		if err := writeRepositoryInfos(infoPath, infos); err != nil {
			return nil, err
		}
	}
	if changedIDs || hasLegacyInfo {
		if err := SaveConfig(&cfg); err != nil {
			return nil, err
		}
	}
	return &cfg, nil
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg *AppConfig) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}
	infoPath, err := repositoryInfoPath()
	if err != nil {
		return err
	}
	return saveConfigFiles(configPath, infoPath, cfg)
}

func saveConfigFiles(configPath, infoPath string, cfg *AppConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return err
	}
	return pruneRepositoryInfos(infoPath, cfg.Repositories)
}

func isUUID(id string) bool {
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(id, "-", ""))
	return err == nil
}
