package catalog

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/transform"
)

var ErrInvalidPOM = errors.New("invalid Maven POM")

// ParsePath 从物理路径中提取 Maven 坐标
func ParsePath(repoPath, filePath string) (groupId, artifactId, version string, ok bool) {
	rel, err := filepath.Rel(repoPath, filePath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", "", false
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	// 最少需要：group(1)/artifact(1)/version(1)/file.jar => 4 段
	if len(parts) < 4 {
		return "", "", "", false
	}

	// 最后一段是文件名，倒数第二段是版本号，倒数第三段是 artifactId
	fileName := parts[len(parts)-1]
	version = parts[len(parts)-2]
	artifactId = parts[len(parts)-3]

	// 剩下的部分组成 groupId
	groupIdParts := parts[:len(parts)-3]
	groupId = strings.Join(groupIdParts, ".")

	// 简单校验文件名是否包含 artifactId（避免误判未知目录）
	if !strings.Contains(fileName, artifactId) {
		return "", "", "", false
	}

	return groupId, artifactId, version, true
}

// ReadPackaging returns a POM's packaging, defaulting to Maven's jar packaging.
func ReadPackaging(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var project struct {
		XMLName   xml.Name `xml:"project"`
		Packaging string   `xml:"packaging"`
	}
	entities := make(map[string]string)
	data = escapeLegacyEntityReferences(data, entities)
	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Entity = entities
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		encoding, err := htmlindex.Get(charset)
		if err != nil {
			return nil, fmt.Errorf("unsupported XML encoding %q: %w", charset, err)
		}
		return transform.NewReader(input, encoding.NewDecoder()), nil
	}
	if err := decoder.Decode(&project); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidPOM, err)
	}
	if project.XMLName.Local != "project" {
		return "", fmt.Errorf("%w: unexpected root element <%s>", ErrInvalidPOM, project.XMLName.Local)
	}

	packaging := strings.TrimSpace(project.Packaging)
	if packaging == "" {
		return "jar", nil
	}
	return packaging, nil
}

func escapeLegacyEntityReferences(data []byte, entities map[string]string) []byte {
	var escaped bytes.Buffer
	for i := 0; i < len(data); {
		if data[i] != '&' {
			escaped.WriteByte(data[i])
			i++
			continue
		}

		end, name, valid := scanEntityReference(data, i)
		if valid {
			escaped.Write(data[i : end+1])
			if name != "" {
				entities[name] = string(data[i : end+1])
			}
			i = end + 1
			continue
		}

		escaped.WriteString("&amp;")
		i++
	}
	return escaped.Bytes()
}

func scanEntityReference(data []byte, amp int) (end int, name string, valid bool) {
	start := amp + 1
	if start >= len(data) {
		return 0, "", false
	}
	if data[start] == '#' {
		i := start + 1
		if i < len(data) && (data[i] == 'x' || data[i] == 'X') {
			i++
			digitStart := i
			for i < len(data) && isHexDigit(data[i]) {
				i++
			}
			if i == digitStart {
				return 0, "", false
			}
		} else {
			digitStart := i
			for i < len(data) && data[i] >= '0' && data[i] <= '9' {
				i++
			}
			if i == digitStart {
				return 0, "", false
			}
		}
		if i < len(data) && data[i] == ';' {
			return i, "", true
		}
		return 0, "", false
	}
	if !isEntityNameStart(data[start]) {
		return 0, "", false
	}
	i := start + 1
	for i < len(data) && isEntityNameChar(data[i]) {
		i++
	}
	if i < len(data) && data[i] == ';' {
		name = string(data[start:i])
		switch name {
		case "amp", "lt", "gt", "apos", "quot":
			return i, "", true
		default:
			return i, name, true
		}
	}
	return 0, "", false
}

func isEntityNameStart(b byte) bool {
	return b == '_' || b == ':' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func isEntityNameChar(b byte) bool {
	return isEntityNameStart(b) || b == '-' || b == '.' || b >= '0' && b <= '9'
}

func isHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}

// IsJarMissing reports whether the JAR alongside a POM is missing.
func IsJarMissing(pomPath string) (bool, error) {
	jarPath := strings.TrimSuffix(pomPath, ".pom") + ".jar"
	info, err := os.Stat(jarPath)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !info.Mode().IsRegular(), nil
}
