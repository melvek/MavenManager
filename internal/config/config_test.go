package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mvnmanager/internal/catalog"
)

func TestNewRepositoryIDIsUUID(t *testing.T) {
	id, err := NewRepositoryID()
	if err != nil {
		t.Fatal(err)
	}
	if !isUUID(id) || id[14] != '4' {
		t.Errorf("NewRepositoryID() = %q, want a version 4 UUID", id)
	}
}

func TestRepositoryInfoIsIndependentAndPrunedWhenRepositoryIsRemoved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "repository-info.json")
	configPath := filepath.Join(t.TempDir(), "config.json")
	info := RepositoryInfo{
		Overview: &catalog.RepositoryOverview{
			TotalFiles: 42, TotalGroups: 3, TotalPackages: 7, TotalVersions: 11,
		},
		LastScannedAt: "2026-01-02T03:04:05Z",
		LastCleanupAt: "2026-01-03T03:04:05Z",
	}
	infos := map[string]RepositoryInfo{"repo-a": info, "repo-b": {}}
	if err := writeRepositoryInfos(path, infos); err != nil {
		t.Fatal(err)
	}
	loaded, err := readRepositoryInfos(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded["repo-a"].Overview == nil ||
		loaded["repo-a"].Overview.TotalFiles != 42 ||
		loaded["repo-a"].LastScannedAt != info.LastScannedAt ||
		loaded["repo-a"].LastCleanupAt != info.LastCleanupAt {
		t.Fatalf("repository info did not round-trip: %+v", loaded["repo-a"])
	}

	config := &AppConfig{Repositories: []Repository{{ID: "repo-a"}}}
	if err := saveConfigFiles(configPath, path, config); err != nil {
		t.Fatal(err)
	}
	loaded, err = readRepositoryInfos(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 {
		t.Fatalf("pruned info contains %d repositories, want 1", len(loaded))
	}
	if _, exists := loaded["repo-b"]; exists {
		t.Fatal("repository info for removed repository was not deleted")
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config was not saved: %v", err)
	}
}

func TestAppConfigDoesNotSerializeRepositoryInfo(t *testing.T) {
	data, err := json.Marshal(AppConfig{
		MavenPath:          "mvn",
		Repositories:       []Repository{{ID: "repo-a", Name: "Local", Path: `C:\m2\repository`}},
		ActiveRepoID:       "repo-a",
		RemoteRepositories: []RemoteRepository{{Name: "Mirror", URL: "https://repo.example/maven"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "overview") ||
		strings.Contains(string(data), "lastScannedAt") ||
		strings.Contains(string(data), "lastCleanupAt") {
		t.Errorf("app config contains repository info fields: %s", data)
	}
	var restored AppConfig
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.RemoteRepositories) != 1 ||
		restored.RemoteRepositories[0].URL != "https://repo.example/maven" {
		t.Errorf("remote repository config did not round-trip: %+v", restored.RemoteRepositories)
	}
}
