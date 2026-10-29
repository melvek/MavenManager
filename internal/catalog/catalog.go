package catalog

import "time"

// 左侧树节点
type TreeNode struct {
	ID         string      `json:"id"`
	Label      string      `json:"label"`                // 显示的名称
	IsLeaf     bool        `json:"isLeaf"`               // 是否叶子节点（artifactId）
	GroupId    string      `json:"groupId,omitempty"`    // 仅叶子节点有
	ArtifactId string      `json:"artifactId,omitempty"` // 仅叶子节点有
	Children   []*TreeNode `json:"children,omitempty"`   // 子节点
}

type RepositoryOverview struct {
	TotalFiles    int64 `json:"totalFiles"`
	TotalGroups   int   `json:"totalGroups"`
	TotalPackages int   `json:"totalPackages"`
	TotalVersions int   `json:"totalVersions"`
}

type ArtifactCoordinate struct {
	GroupID    string `json:"groupId"`
	ArtifactID string `json:"artifactId"`
	Version    string `json:"version"`
}

type ArtifactVersionSummary struct {
	GroupID    string    `json:"groupId"`
	ArtifactID string    `json:"artifactId"`
	Version    string    `json:"version"`
	ModTime    time.Time `json:"modTime"`
}

// 右侧版本详情
type VersionDetail struct {
	Version   string    `json:"version"`
	FileName  string    `json:"fileName"`
	Size      int64     `json:"size"`
	ModTime   time.Time `json:"modTime"`
	Path      string    `json:"path"` // 用于删除
	Packaging string    `json:"packaging,omitempty"`
	// 后续扩展：是否有 .lastUpdated，是否被引用等
}

type GarbageItem struct {
	ID           string    `json:"id"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	VersionSize  int64     `json:"versionSize,omitempty"`
	Category     string    `json:"category"` // 分类：如 lastUpdated, remote, emptyDir 等
	Reason       string    `json:"reason"`   // 提示信息
	ModTime      time.Time `json:"modTime"`
	IsDiagnostic bool      `json:"isDiagnostic,omitempty"`
	PackageName  string    `json:"packageName"`
	GroupId      string    `json:"groupId,omitempty"`
	ArtifactId   string    `json:"artifactId,omitempty"`
	Version      string    `json:"version,omitempty"`
}
