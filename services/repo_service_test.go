package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"mvnmanager/internal/catalog"
	"mvnmanager/internal/config"
)

func TestScanGarbageDetectsMissingJarOnlyWhenEnabled(t *testing.T) {
	repoPath := t.TempDir()
	pomPath := filepath.Join(repoPath, "com", "example", "library", "1.0", "library-1.0.pom")
	if err := os.MkdirAll(filepath.Dir(pomPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pomPath, []byte(`<project><modelVersion>4.0.0</modelVersion></project>`), 0600); err != nil {
		t.Fatal(err)
	}

	service := &RepoService{}
	results := service.ScanGarbage(repoPath, map[string]bool{"missingJar": false})
	if len(results) != 0 {
		t.Fatalf("ScanGarbage() with missingJar disabled returned %d results, want 0", len(results))
	}

	results = service.ScanGarbage(repoPath, map[string]bool{"missingJar": true})
	if len(results) != 1 {
		t.Fatalf("ScanGarbage() returned %d results, want 1", len(results))
	}
	if results[0].Category != "missingJar" || !results[0].IsDiagnostic || results[0].Path != pomPath {
		t.Errorf("ScanGarbage() result = %+v, want a missingJar diagnostic for the POM", results[0])
	}
	if results[0].PackageName != "library-1.0" ||
		results[0].GroupId != "com.example" ||
		results[0].ArtifactId != "library" ||
		results[0].Version != "1.0" {
		t.Errorf("ScanGarbage() Maven coordinates = %+v, want com.example:library:1.0", results[0])
	}

	jarPath := filepath.Join(filepath.Dir(pomPath), "library-1.0.jar")
	if err := os.WriteFile(jarPath, []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}
	results = service.ScanGarbage(repoPath, map[string]bool{"missingJar": true})
	if len(results) != 0 {
		t.Fatalf("ScanGarbage() with matching JAR present returned %d results, want 0", len(results))
	}
}

func TestScanGarbageSkipsNonJarPackaging(t *testing.T) {
	repoPath := t.TempDir()
	pomPath := filepath.Join(repoPath, "com", "example", "library", "1.0", "library-1.0.pom")
	if err := os.MkdirAll(filepath.Dir(pomPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pomPath, []byte(`<project><packaging>pom</packaging></project>`), 0600); err != nil {
		t.Fatal(err)
	}

	results := (&RepoService{}).ScanGarbage(repoPath, map[string]bool{"missingJar": true})
	if len(results) != 0 {
		t.Fatalf("ScanGarbage() for pom packaging returned %d results, want 0", len(results))
	}
}

func TestScanGarbageIgnoresInvalidPomFiles(t *testing.T) {
	repoPath := t.TempDir()
	poms := map[string]string{
		filepath.Join("com", "example", "html-error", "1.0", "html-error-1.0.pom"): `<!DOCTYPE html><html><head><meta name="csrf_token"></head><body>Access denied</body></html>`,
		filepath.Join("com", "example", "malformed", "1.0", "malformed-1.0.pom"):   `<project><version>1.0</project>`,
	}
	for relativePath, contents := range poms {
		path := filepath.Join(repoPath, relativePath)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	results := (&RepoService{}).ScanGarbage(repoPath, map[string]bool{"missingJar": true})
	if len(results) != 0 {
		t.Errorf("ScanGarbage() returned results for invalid POM files: %+v", results)
	}
}

func TestScanGarbageReportsMavenVersionDirectorySize(t *testing.T) {
	repoPath := t.TempDir()
	versionDir := filepath.Join(repoPath, "com", "example", "library", "1.0")
	if err := os.MkdirAll(versionDir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"library-1.0.jar":         "jar-content",
		"library-1.0.pom":         "pom-content",
		"library-1.0.lastUpdated": "failure-marker",
	}
	var expectedSize int64
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(versionDir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
		expectedSize += int64(len(content))
	}

	results := (&RepoService{}).ScanGarbage(repoPath, map[string]bool{"lastUpdated": true})
	if len(results) != 1 {
		t.Fatalf("ScanGarbage() returned %d results, want 1", len(results))
	}
	if results[0].Size != int64(len(files["library-1.0.lastUpdated"])) {
		t.Errorf("garbage file size = %d, want %d", results[0].Size, len(files["library-1.0.lastUpdated"]))
	}
	if results[0].VersionSize != expectedSize {
		t.Errorf("Maven version size = %d, want %d", results[0].VersionSize, expectedSize)
	}
}

func TestScanGarbageStreamsConcurrentBatchesAndProgress(t *testing.T) {
	repoPath := t.TempDir()
	const fileCount = 55
	for i := 0; i < fileCount; i++ {
		path := filepath.Join(repoPath, fmt.Sprintf("failed-%d.lastUpdated", i))
		if err := os.WriteFile(path, nil, 0600); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	resultCount := 0
	progressReported := false
	scanned, total, err := (&RepoService{}).scanGarbage(
		repoPath,
		map[string]bool{"lastUpdated": true},
		func(phase string, count, total int64, _ string) {
			if count >= 50 {
				mu.Lock()
				progressReported = true
				mu.Unlock()
			}
			if phase == "processing" && total != fileCount {
				t.Errorf("processing total = %d, want %d", total, fileCount)
			}
		},
		func(batch []catalog.GarbageItem) {
			mu.Lock()
			resultCount += len(batch)
			mu.Unlock()
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if scanned != fileCount {
		t.Errorf("scanned files = %d, want %d", scanned, fileCount)
	}
	if total != fileCount {
		t.Errorf("total files = %d, want %d", total, fileCount)
	}
	mu.Lock()
	defer mu.Unlock()
	if resultCount != fileCount {
		t.Errorf("streamed result count = %d, want %d", resultCount, fileCount)
	}
	if !progressReported {
		t.Error("scan progress was not reported")
	}
}

func TestBuildRepositoryOverviewCountsPackagesAndVersions(t *testing.T) {
	versions := map[string][]catalog.VersionDetail{
		"com.example:alpha": {
			{Version: "1.0", FileName: "alpha-1.0.pom"},
			{Version: "1.0", FileName: "alpha-1.0.jar"},
			{Version: "2.0", FileName: "alpha-2.0.pom"},
		},
		"org.example:beta": {
			{Version: "1.0", FileName: "beta-1.0.pom"},
		},
	}

	got := buildRepositoryOverview(42, versions)
	if got.TotalFiles != 42 || got.TotalGroups != 2 || got.TotalPackages != 2 || got.TotalVersions != 3 {
		t.Errorf("buildRepositoryOverview() = %+v, want 42 files, 2 groups, 2 packages, 3 versions", got)
	}
}

func TestDeleteFilesSupportsPermanentDeleteAndKeepsRepoRoot(t *testing.T) {
	repoPath := t.TempDir()
	target := filepath.Join(repoPath, "artifact.lastUpdated")
	if err := os.WriteFile(target, []byte("failed"), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := (&RepoService{}).DeleteFiles(repoPath, []string{target}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("permanent delete left the selected file: %v", err)
	}
	if result["count"] != 1 {
		t.Errorf("deleted count = %v, want 1", result["count"])
	}

	result, err = (&RepoService{}).DeleteFiles(repoPath, []string{repoPath}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result["count"] != 0 || result["failures"] != 1 {
		t.Errorf("deleting repo root returned %v, want zero deletes and one failure", result)
	}
	if _, err := os.Stat(repoPath); err != nil {
		t.Fatalf("repository root must remain: %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	result, err = (&RepoService{}).DeleteFiles(repoPath, []string{outside}, false)
	if err != nil {
		t.Fatal(err)
	}
	if result["count"] != 0 || result["failures"] != 1 {
		t.Errorf("deleting outside path returned %v, want zero deletes and one failure", result)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file must remain: %v", err)
	}
}

func TestDeleteArtifactVersionRemovesWholeVersionDirectory(t *testing.T) {
	repoPath := t.TempDir()
	versionDir := filepath.Join(repoPath, "com", "example", "library", "1.0")
	if err := os.MkdirAll(versionDir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"library-1.0.pom", "library-1.0.jar", "library-1.0.pom.lastUpdated"} {
		if err := os.WriteFile(filepath.Join(versionDir, name), []byte("data"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := (&RepoService{}).DeleteArtifactVersion(repoPath, "com.example", "library", "1.0", false)
	if err != nil {
		t.Fatal(err)
	}
	if result["count"].(int) < 1 {
		t.Errorf("deleted count = %v, want deleted version directory", result["count"])
	}
	if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
		t.Fatalf("version directory still exists: %v", err)
	}
}

func TestDeleteArtifactVersionRejectsInvalidCoordinates(t *testing.T) {
	repoPath := t.TempDir()
	if _, err := (&RepoService{}).DeleteArtifactVersion(repoPath, "..", "library", "1.0", false); err == nil {
		t.Fatal("invalid group ID was accepted")
	}
	if _, err := (&RepoService{}).DeleteArtifactVersion(repoPath, "com.example", "library", "..", false); err == nil {
		t.Fatal("invalid version was accepted")
	}
}

func TestValidateRemoteRepository(t *testing.T) {
	if _, err := validateRemoteRepository("https://repo.example/maven"); err != nil {
		t.Fatalf("valid repository URL rejected: %v", err)
	}
	for _, rawURL := range []string{
		"file:///tmp/repository",
		"https://user:password@repo.example/maven",
		"https://repo.example/maven?token=secret",
		"https://repo.example/path%20with%20space",
		"not a URL",
	} {
		if _, err := validateRemoteRepository(rawURL); err == nil {
			t.Errorf("invalid repository URL %q was accepted", rawURL)
		}
	}
}

func TestBuildMavenRepairArgumentsUseLocalRepositoryAndExtraRemotes(t *testing.T) {
	args, err := buildMavenRepairArguments(
		`C:\m2\repository`,
		"com.example",
		"library",
		"1.2.3",
		[]config.RemoteRepository{{Name: "Mirror", URL: "https://repo.example/maven"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"org.apache.maven.plugins:maven-dependency-plugin:3.1.2:get",
		"-Dartifact=com.example:library:1.2.3",
		"-Dtransitive=false",
		"-Dmaven.repo.local=C:\\m2\\repository",
		"-DremoteRepositories=app-repo-1::default::https://repo.example/maven",
	}
	if len(args) != len(want) {
		t.Fatalf("Maven args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("Maven arg %d = %q, want %q", i, args[i], want[i])
		}
	}
}

func TestRefreshArtifactVersionUpdatesCachedFiles(t *testing.T) {
	repoPath := t.TempDir()
	versionDir := filepath.Join(repoPath, "com", "example", "library", "1.0")
	if err := os.MkdirAll(versionDir, 0700); err != nil {
		t.Fatal(err)
	}
	pomPath := filepath.Join(versionDir, "library-1.0.pom")
	jarPath := filepath.Join(versionDir, "library-1.0.jar")
	if err := os.WriteFile(pomPath, []byte("<project/>"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jarPath, []byte("jar-data"), 0600); err != nil {
		t.Fatal(err)
	}

	service := &RepoService{
		versions: map[string][]catalog.VersionDetail{
			"com.example:library": {
				{Version: "1.0", FileName: "library-1.0.pom"},
				{Version: "2.0", FileName: "library-2.0.pom"},
			},
		},
	}
	service.refreshArtifactVersion(repoPath, "com.example", "library", "1.0")

	got := service.GetArtifactVersions("com.example", "library")
	if len(got) != 3 {
		t.Fatalf("cached artifact files = %d, want 3", len(got))
	}
	foundJar := false
	foundOtherVersion := false
	for _, file := range got {
		if file.Version == "1.0" && file.FileName == "library-1.0.jar" && file.Size == int64(len("jar-data")) {
			foundJar = true
		}
		if file.Version == "2.0" {
			foundOtherVersion = true
		}
	}
	if !foundJar {
		t.Error("refreshed cache does not include repaired JAR file")
	}
	if !foundOtherVersion {
		t.Error("refresh removed files belonging to another version")
	}
}

func TestResolveMavenInstallationUsesConfSettings(t *testing.T) {
	mavenHome := t.TempDir()
	executableName := "mvn"
	if runtime.GOOS == "windows" {
		executableName = "mvn.cmd"
	}
	executablePath := filepath.Join(mavenHome, "bin", executableName)
	settingsPath := filepath.Join(mavenHome, "conf", "settings.xml")
	if err := os.MkdirAll(filepath.Dir(executablePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executablePath, []byte{}, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("<settings/>"), 0600); err != nil {
		t.Fatal(err)
	}

	gotExecutable, gotSettings, err := resolveMavenInstallation(mavenHome)
	if err != nil {
		t.Fatal(err)
	}
	if gotExecutable != executablePath {
		t.Errorf("Maven executable = %q, want %q", gotExecutable, executablePath)
	}
	if gotSettings != settingsPath {
		t.Errorf("Maven settings = %q, want %q", gotSettings, settingsPath)
	}
}

func TestMavenCommandHandlesWindowsBatchPathsWithSpaces(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows batch command invocation")
	}

	batchPath := filepath.Join(t.TempDir(), "maven home", "bin", "mvn.cmd")
	if err := os.MkdirAll(filepath.Dir(batchPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(batchPath, []byte("@echo off\r\necho probe-ok\r\n"), 0600); err != nil {
		t.Fatal(err)
	}

	cmd, err := mavenCommand(context.Background(), batchPath, "test-goal")
	if err != nil {
		t.Fatal(err)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("batch command failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "probe-ok") {
		t.Fatalf("batch command output = %q, want probe-ok", output)
	}
}
