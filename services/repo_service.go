package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mvnmanager/internal/catalog"
	"mvnmanager/internal/config"

	"github.com/Bios-Marcel/wastebasket/v2"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type RepoService struct {
	ctx context.Context
	mu  sync.RWMutex

	repoPath string
	versions map[string][]catalog.VersionDetail
	overview catalog.RepositoryOverview
}

func (s *RepoService) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	s.ctx = ctx
	s.versions = make(map[string][]catalog.VersionDetail)
	s.overview = catalog.RepositoryOverview{}
	return nil
}

func (s *RepoService) LoadAssets(repoPath string) error {
	info, err := os.Stat(repoPath)
	if err != nil {
		return fmt.Errorf("无法访问仓库目录: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("仓库路径不是目录: %s", repoPath)
	}

	app := application.Get()
	if app == nil {
		return fmt.Errorf("Wails 应用尚未启动")
	}

	go func() {
		s.mu.Lock()
		s.repoPath = repoPath
		s.versions = make(map[string][]catalog.VersionDetail)
		s.overview = catalog.RepositoryOverview{}
		s.mu.Unlock()

		app.Event.Emit("load_assets_progress", map[string]any{
			"phase": "counting", "scannedFiles": 0, "totalFiles": 0, "currentPath": repoPath,
		})
		totalFiles, err := countRepositoryFiles(repoPath, func(count int64, path string) {
			app.Event.Emit("load_assets_progress", map[string]any{
				"phase": "counting", "scannedFiles": count, "totalFiles": 0, "currentPath": path,
			})
		})
		if err != nil {
			app.Event.Emit("load_assets_error", map[string]string{"message": err.Error()})
			return
		}

		app.Event.Emit("load_assets_progress", map[string]any{
			"phase": "processing", "scannedFiles": 0, "totalFiles": totalFiles, "currentPath": repoPath,
		})
		var count atomic.Int64
		err = filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			currentCount := count.Add(1)
			if currentCount%1000 == 0 {
				app.Event.Emit("load_assets_progress", map[string]any{
					"phase": "processing", "scannedFiles": currentCount,
					"totalFiles": totalFiles, "currentPath": path,
				})
			}
			name := d.Name()
			if strings.HasSuffix(name, ".jar") || strings.HasSuffix(name, ".pom") {
				g, a, v, ok := catalog.ParsePath(repoPath, path)
				if ok {
					info, err := d.Info()
					if err != nil {
						return err
					}
					key := fmt.Sprintf("%s:%s", g, a)
					s.mu.Lock()
					s.versions[key] = append(s.versions[key], catalog.VersionDetail{
						Version: v, FileName: name, Size: info.Size(), ModTime: info.ModTime(), Path: path,
					})
					s.mu.Unlock()
				}
			}
			return nil
		})
		if err != nil {
			app.Event.Emit("load_assets_error", map[string]string{"message": err.Error()})
			return
		}

		s.mu.Lock()
		overview := buildRepositoryOverview(totalFiles, s.versions)
		s.overview = overview
		s.mu.Unlock()

		log.Printf("依赖树加载完成，共扫描 %d 个文件、%d 个包、%d 个版本", count.Load(), overview.TotalPackages, overview.TotalVersions)
		app.Event.Emit("load_assets_progress", map[string]any{
			"phase": "processing", "scannedFiles": count.Load(),
			"totalFiles": totalFiles, "currentPath": "",
		})
		app.Event.Emit("load_assets_done", overview)
	}()
	return nil
}

func (s *RepoService) GetOverview() catalog.RepositoryOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.overview
}

func buildRepositoryOverview(totalFiles int64, versions map[string][]catalog.VersionDetail) catalog.RepositoryOverview {
	seenVersions := make(map[string]struct{})
	seenGroups := make(map[string]struct{})
	for key, details := range versions {
		groupID, _, _ := strings.Cut(key, ":")
		seenGroups[groupID] = struct{}{}
		for _, detail := range details {
			seenVersions[key+":"+detail.Version] = struct{}{}
		}
	}
	return catalog.RepositoryOverview{
		TotalFiles:    totalFiles,
		TotalGroups:   len(seenGroups),
		TotalPackages: len(versions),
		TotalVersions: len(seenVersions),
	}
}

func (s *RepoService) GetTree() []*catalog.TreeNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	groupMap := make(map[string]*catalog.TreeNode)

	for key := range s.versions {
		parts := strings.Split(key, ":")
		g, a := parts[0], parts[1]

		if _, exists := groupMap[g]; !exists {
			groupMap[g] = &catalog.TreeNode{
				ID:       g,
				Label:    g,
				IsLeaf:   false,
				Children: []*catalog.TreeNode{},
			}
		}

		artifactUniqueId := fmt.Sprintf("%s:%s", g, a)
		groupMap[g].Children = append(groupMap[g].Children, &catalog.TreeNode{
			ID:         artifactUniqueId,
			Label:      a,
			IsLeaf:     true,
			GroupId:    g,
			ArtifactId: a,
		})
	}

	// 直接构建返回的顶级节点切片
	res := make([]*catalog.TreeNode, 0, len(groupMap))
	for _, node := range groupMap {
		// 对每个 Group 下的 Artifact 按字母排序
		sort.Slice(node.Children, func(i, j int) bool {
			return node.Children[i].Label < node.Children[j].Label
		})
		res = append(res, node)
	}

	// 对顶级的 GroupId 按字母排序
	sort.Slice(res, func(i, j int) bool {
		return res[i].Label < res[j].Label
	})

	return res
}

func (s *RepoService) GetArtifactVersions(groupId, artifactId string) []catalog.VersionDetail {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", groupId, artifactId)
	return s.versions[key]
}

func (s *RepoService) ScanGarbage(repoPath string, options map[string]bool) []catalog.GarbageItem {
	var results []catalog.GarbageItem
	var resultsMu sync.Mutex
	_, _, err := s.scanGarbage(repoPath, options, nil, func(batch []catalog.GarbageItem) {
		resultsMu.Lock()
		results = append(results, batch...)
		resultsMu.Unlock()
	})
	if err != nil {
		log.Printf("清理扫描失败: %v", err)
	}
	return results
}

type cleanupScanProgress struct {
	ScannedFiles int64  `json:"scannedFiles"`
	TotalFiles   int64  `json:"totalFiles"`
	Phase        string `json:"phase"`
	CurrentPath  string `json:"currentPath"`
}

type cleanupScanResult struct {
	Items []catalog.GarbageItem `json:"items"`
}

type cleanupScanDone struct {
	ScannedFiles int64 `json:"scannedFiles"`
	TotalFiles   int64 `json:"totalFiles"`
	ResultCount  int   `json:"resultCount"`
}

type cleanupScanError struct {
	Message string `json:"message"`
}

// StartGarbageScan scans a repository in the background and emits progress and result batches.
func (s *RepoService) StartGarbageScan(repoPath string, options map[string]bool) error {
	info, err := os.Stat(repoPath)
	if err != nil {
		return fmt.Errorf("无法访问仓库目录: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("仓库路径不是目录: %s", repoPath)
	}

	app := application.Get()
	if app == nil {
		return fmt.Errorf("Wails 应用尚未启动")
	}
	app.Event.Emit("cleanup_scan_progress", cleanupScanProgress{
		Phase: "counting", CurrentPath: repoPath,
	})

	go func() {
		var resultCount atomic.Int64
		scanned, total, err := s.scanGarbage(repoPath, options, func(phase string, scanned, total int64, path string) {
			app.Event.Emit("cleanup_scan_progress", cleanupScanProgress{
				ScannedFiles: scanned, TotalFiles: total,
				Phase: phase, CurrentPath: path,
			})
		}, func(batch []catalog.GarbageItem) {
			resultCount.Add(int64(len(batch)))
			app.Event.Emit("cleanup_scan_results", cleanupScanResult{Items: batch})
		})
		if err != nil {
			app.Event.Emit("cleanup_scan_error", cleanupScanError{Message: err.Error()})
			return
		}
		app.Event.Emit("cleanup_scan_done", cleanupScanDone{
			ScannedFiles: scanned, TotalFiles: total,
			ResultCount: int(resultCount.Load()),
		})
	}()
	return nil
}

func (s *RepoService) scanGarbage(
	repoPath string,
	options map[string]bool,
	onProgress func(string, int64, int64, string),
	onResults func([]catalog.GarbageItem),
) (int64, int64, error) {
	totalFiles, err := countRepositoryFiles(repoPath, func(scanned int64, path string) {
		if onProgress != nil {
			onProgress("counting", scanned, 0, path)
		}
	})
	if err != nil {
		return 0, 0, err
	}
	if onProgress != nil {
		onProgress("processing", 0, totalFiles, repoPath)
	}

	workerCount := runtime.NumCPU()
	if workerCount > 8 {
		workerCount = 8
	}
	if workerCount < 1 {
		workerCount = 1
	}

	type scanJob struct {
		path string
		name string
	}
	jobs := make(chan scanJob, workerCount*2)
	var scanned atomic.Int64
	var versionSizes sync.Map
	var progressMu sync.Mutex
	currentPath := ""
	var batchMu sync.Mutex
	var batch []catalog.GarbageItem
	const batchSize = 20

	emitBatch := func(items []catalog.GarbageItem) {
		if len(items) > 0 && onResults != nil {
			onResults(items)
		}
	}
	flushBatch := func() {
		batchMu.Lock()
		items := batch
		batch = nil
		batchMu.Unlock()
		emitBatch(items)
	}
	stopFlusher := make(chan struct{})
	flusherDone := make(chan struct{})
	go func() {
		defer close(flusherDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				flushBatch()
				if onProgress != nil {
					progressMu.Lock()
					onProgress("processing", scanned.Load(), totalFiles, currentPath)
					progressMu.Unlock()
				}
			case <-stopFlusher:
				return
			}
		}
	}()
	addResult := func(item catalog.GarbageItem) {
		batchMu.Lock()
		batch = append(batch, item)
		if len(batch) < batchSize {
			batchMu.Unlock()
			return
		}
		items := batch
		batch = nil
		batchMu.Unlock()
		emitBatch(items)
	}

	dirs := make([]string, 0)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for range workerCount {
		go func() {
			defer workers.Done()
			for job := range jobs {
				progressMu.Lock()
				currentPath = job.path
				progressMu.Unlock()

				info, err := os.Stat(job.path)
				if err != nil {
					log.Printf("无法读取扫描文件: %s, 错误: %v", job.path, err)
				} else {
					var category, reason string
					if options["lastUpdated"] && strings.HasSuffix(job.name, ".lastUpdated") {
						category, reason = "lastUpdated", "下载失败标记"
					} else if options["remote"] && job.name == "_remote.repositories" {
						category, reason = "remote", "仓库来源记录"
					} else if options["inProgress"] && strings.HasSuffix(job.name, ".in-progress") {
						category, reason = "inProgress", "中断的下载残留"
					} else if options["zeroByte"] && info.Size() == 0 && (strings.HasSuffix(job.name, ".jar") || strings.HasSuffix(job.name, ".pom")) {
						category, reason = "zeroByte", "零字节无效文件"
					}

					if category != "" {
						addResult(newGarbageItem(repoPath, job.path, job.name, info.Size(), category, reason, info.ModTime(), &versionSizes))
					} else if options["missingJar"] && strings.HasSuffix(job.name, ".pom") {
						_, _, _, ok := catalog.ParsePath(repoPath, job.path)
						if ok {
							packaging, err := catalog.ReadPackaging(job.path)
							if err != nil {
								if !errors.Is(err, catalog.ErrInvalidPOM) {
									log.Printf("无法读取 POM packaging: %s, 错误: %v", job.path, err)
								}
							} else if packaging == "jar" {
								missing, err := catalog.IsJarMissing(job.path)
								if err != nil {
									log.Printf("无法检查对应 JAR: %s, 错误: %v", strings.TrimSuffix(job.path, ".pom")+".jar", err)
								} else if missing {
									item := newGarbageItem(repoPath, job.path, job.name, 0, "missingJar",
										"POM 声明需要 JAR，但未找到对应 JAR", info.ModTime(), &versionSizes)
									item.IsDiagnostic = true
									addResult(item)
								}
							}
						}
					}
				}

				count := scanned.Add(1)
				progressMu.Lock()
				if count%50 == 0 && onProgress != nil {
					onProgress("processing", count, totalFiles, currentPath)
				}
				progressMu.Unlock()
			}
		}()
	}

	walkErr := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		jobs <- scanJob{path: path, name: d.Name()}
		return nil
	})
	close(jobs)
	workers.Wait()
	if walkErr != nil {
		close(stopFlusher)
		<-flusherDone
		flushBatch()
		return scanned.Load(), totalFiles, walkErr
	}

	if options["emptyDir"] {
		for i := len(dirs) - 1; i >= 0; i-- {
			dir := dirs[i]
			if dir == repoPath {
				continue
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				log.Printf("无法检查目录是否为空: %s, 错误: %v", dir, err)
				continue
			}
			if len(entries) == 0 {
				info, err := os.Stat(dir)
				if err != nil {
					log.Printf("无法读取目录信息: %s, 错误: %v", dir, err)
					continue
				}
				addResult(catalog.GarbageItem{
					ID: dir, Path: dir, Size: 0, Category: "emptyDir", Reason: "空文件夹", ModTime: info.ModTime(),
					PackageName: filepath.Base(dir),
				})
			}
			progressMu.Lock()
			currentPath = dir
			progressMu.Unlock()
		}
	}
	close(stopFlusher)
	<-flusherDone
	flushBatch()
	if onProgress != nil {
		progressMu.Lock()
		onProgress("processing", scanned.Load(), totalFiles, "")
		progressMu.Unlock()
	}
	return scanned.Load(), totalFiles, nil
}

func newGarbageItem(repoPath, path, fileName string, size int64, category, reason string, modTime time.Time, versionSizes *sync.Map) catalog.GarbageItem {
	item := catalog.GarbageItem{
		ID: path, Path: path, Size: size, Category: category, Reason: reason,
		ModTime: modTime, PackageName: fileName,
	}
	if groupID, artifactID, version, ok := catalog.ParsePath(repoPath, path); ok {
		item.PackageName = artifactID + "-" + version
		item.GroupId = groupID
		item.ArtifactId = artifactID
		item.Version = version
		item.VersionSize = getVersionDirectorySize(filepath.Dir(path), versionSizes)
	}
	return item
}

func getVersionDirectorySize(versionDir string, cache *sync.Map) int64 {
	if size, ok := cache.Load(versionDir); ok {
		return size.(int64)
	}

	var size int64
	err := filepath.WalkDir(versionDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	if err != nil {
		log.Printf("无法计算版本目录大小: %s, 错误: %v", versionDir, err)
		return 0
	}
	actual, _ := cache.LoadOrStore(versionDir, size)
	return actual.(int64)
}

func countRepositoryFiles(repoPath string, onProgress func(int64, string)) (int64, error) {
	var count int64
	err := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		count++
		if onProgress != nil && count%1000 == 0 {
			onProgress(count, path)
		}
		return nil
	})
	return count, err
}

// DeleteFiles 将文件移入回收站，并自动向上递归清理变空的父目录
// DeleteFiles 直接调用系统回收站，并自动向上递归清理变空的父目录
func (s *RepoService) DeleteFiles(repoPath string, paths []string, moveToTrash bool) (map[string]interface{}, error) {
	cleanedRepoPath, _ := filepath.Abs(repoPath)

	var totalSize int64
	var successCount int
	var failureCount int

	for _, p := range paths {
		absPath, err := filepath.Abs(p)
		if err != nil {
			failureCount++
			continue
		}

		// 安全校验：必须在当前仓库路径下
		relPath, err := filepath.Rel(cleanedRepoPath, absPath)
		if err != nil || relPath == "." || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			failureCount++
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			failureCount++
			continue
		}

		if moveToTrash {
			err = wastebasket.Trash(absPath)
		} else {
			err = os.RemoveAll(absPath)
		}
		if err != nil {
			log.Printf("【清理失败】无法删除: %s, 错误: %v", absPath, err)
			failureCount++
			continue
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}
		successCount++

		// 2. 向上追溯：检查父目录是否变空了
		parentDir := filepath.Dir(absPath)

		for {
			relParent, err := filepath.Rel(cleanedRepoPath, parentDir)
			if err != nil || strings.HasPrefix(relParent, "..") || relParent == "." {
				break // 到达仓库根目录，停止向上
			}

			entries, err := os.ReadDir(parentDir)

			if err == nil && len(entries) == 0 {
				if moveToTrash {
					err = wastebasket.Trash(parentDir)
				} else {
					err = os.Remove(parentDir)
				}
				if err == nil {
					successCount++
					parentDir = filepath.Dir(parentDir) // 继续往上
				} else {
					log.Printf("【清理失败】无法清理空目录: %s, 错误: %v", parentDir, err)
					break
				}
			} else {
				// 目录里还有东西，不空，立刻停止
				break
			}
		}
	}

	return map[string]any{
		"count":    successCount,
		"size":     totalSize,
		"failures": failureCount,
	}, nil
}

func (s *RepoService) DeleteArtifactVersion(repoPath, groupID, artifactID, version string, moveToTrash bool) (map[string]interface{}, error) {
	if !validMavenCoordinate(groupID, artifactID, version) {
		return nil, fmt.Errorf("invalid Maven coordinates")
	}
	groupPath := filepath.Join(strings.Split(groupID, ".")...)
	versionPath := filepath.Join(repoPath, groupPath, artifactID, version)
	return s.DeleteFiles(repoPath, []string{versionPath}, moveToTrash)
}

func (s *RepoService) RepairArtifact(repoPath, groupID, artifactID, version, mavenPath string, remoteRepositories []config.RemoteRepository) error {
	if !validMavenCoordinate(groupID, artifactID, version) {
		return fmt.Errorf("invalid Maven coordinates")
	}
	repoInfo, err := os.Stat(repoPath)
	if err != nil {
		return fmt.Errorf("access local Maven repository: %w", err)
	}
	if !repoInfo.IsDir() {
		return fmt.Errorf("local Maven repository path is not a directory")
	}
	if mavenPath == "" {
		mavenPath = "mvn"
	}
	mavenPath, settingsPath, err := resolveMavenInstallation(mavenPath)
	if err != nil {
		return err
	}

	args, err := buildMavenRepairArguments(repoPath, groupID, artifactID, version, remoteRepositories)
	if err != nil {
		return err
	}
	if settingsPath != "" {
		args = append([]string{"-gs", settingsPath}, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd, err := mavenCommand(ctx, mavenPath, args...)
	if err != nil {
		return err
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		details := strings.TrimSpace(string(output))
		if details == "" {
			return fmt.Errorf("Maven dependency repair failed: %w", err)
		}
		if len(details) > 4000 {
			details = details[len(details)-4000:]
		}
		return fmt.Errorf("Maven dependency repair failed: %w\n%s", err, details)
	}
	return nil
}

func resolveMavenInstallation(mavenPath string) (string, string, error) {
	mavenPath = strings.TrimSpace(mavenPath)
	if mavenPath == "" {
		return "mvn", "", nil
	}

	info, err := os.Stat(mavenPath)
	if err != nil {
		if os.IsNotExist(err) && filepath.Base(mavenPath) == mavenPath {
			return mavenPath, "", nil
		}
		return "", "", fmt.Errorf("access Maven path %q: %w", mavenPath, err)
	}

	var mavenHome string
	if info.IsDir() {
		mavenHome = mavenPath
		executableName := "mvn"
		if runtime.GOOS == "windows" {
			executableName = "mvn.cmd"
		}
		mavenPath = filepath.Join(mavenHome, "bin", executableName)
		if _, err := os.Stat(mavenPath); err != nil {
			return "", "", fmt.Errorf("Maven executable not found under configured root %q: %w", mavenHome, err)
		}
	} else if strings.EqualFold(filepath.Base(filepath.Dir(mavenPath)), "bin") {
		mavenHome = filepath.Dir(filepath.Dir(mavenPath))
	}

	if mavenHome == "" {
		return mavenPath, "", nil
	}
	for _, settingsPath := range []string{
		filepath.Join(mavenHome, "conf", "settings.xml"),
		filepath.Join(mavenHome, "config", "settings.xml"),
	} {
		if _, err := os.Stat(settingsPath); err == nil {
			return mavenPath, settingsPath, nil
		} else if !os.IsNotExist(err) {
			return "", "", fmt.Errorf("access Maven settings file %q: %w", settingsPath, err)
		}
	}
	return mavenPath, "", nil
}

func buildMavenRepairArguments(repoPath, groupID, artifactID, version string, remoteRepositories []config.RemoteRepository) ([]string, error) {
	if !validMavenCoordinate(groupID, artifactID, version) {
		return nil, fmt.Errorf("invalid Maven coordinates")
	}
	args := []string{
		"org.apache.maven.plugins:maven-dependency-plugin:3.1.2:get",
		"-Dartifact=" + groupID + ":" + artifactID + ":" + version,
		"-Dtransitive=false",
		"-Dmaven.repo.local=" + repoPath,
	}
	if len(remoteRepositories) > 0 {
		repositories := make([]string, 0, len(remoteRepositories))
		for i, repository := range remoteRepositories {
			repositoryURL, err := validateRemoteRepository(repository.URL)
			if err != nil {
				return nil, fmt.Errorf("invalid remote repository %q: %w", repository.Name, err)
			}
			repositories = append(repositories, fmt.Sprintf("app-repo-%d::default::%s", i+1, repositoryURL))
		}
		args = append(args, "-DremoteRepositories="+strings.Join(repositories, ","))
	}
	return args, nil
}

func validMavenCoordinate(groupID, artifactID, version string) bool {
	if groupID == "" || artifactID == "" || version == "" {
		return false
	}
	for _, part := range strings.Split(groupID, ".") {
		if !validCoordinatePart(part) {
			return false
		}
	}
	return validCoordinatePart(artifactID) && validCoordinatePart(version)
}

func validCoordinatePart(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' || strings.ContainsRune("._+-", r)) {
			return false
		}
	}
	return true
}

func validateRemoteRepository(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	if (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		strings.ContainsAny(rawURL, "%!,\"") {
		return "", fmt.Errorf("URL must be HTTP(S), without credentials, query, or fragment")
	}
	return parsed.String(), nil
}

func mavenCommand(ctx context.Context, mavenPath string, args ...string) (*exec.Cmd, error) {
	if runtime.GOOS == "windows" && (strings.EqualFold(filepath.Ext(mavenPath), ".cmd") ||
		strings.EqualFold(filepath.Ext(mavenPath), ".bat") || filepath.Ext(mavenPath) == "") {
		commandParts := append([]string{"/d", "/c", "call", mavenPath}, args...)
		return exec.CommandContext(ctx, "cmd.exe", commandParts...), nil
	}
	return exec.CommandContext(ctx, mavenPath, args...), nil
}
