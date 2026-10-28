package services

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mvnmanager/internal/catalog"
)

func TestWriteArtifactExportArchiveIncludesMultipleArtifactsAndSelectedScript(t *testing.T) {
	repoPath := t.TempDir()
	packages := []catalog.ArtifactCoordinate{
		{GroupID: "com.example", ArtifactID: "library", Version: "1.2.3"},
		{GroupID: "org.sample", ArtifactID: "widget", Version: "2.0"},
	}
	files := map[string]string{
		filepath.Join("com", "example", "library", "1.2.3", "library-1.2.3.jar"): "jar-binary",
		filepath.Join("com", "example", "library", "1.2.3", "library-1.2.3.pom"): "<project/>",
		filepath.Join("org", "sample", "widget", "2.0", "widget-2.0.pom"):        "<project/>",
	}
	for relativePath, contents := range files {
		fullPath := filepath.Join(repoPath, relativePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}

	outputPath := filepath.Join(t.TempDir(), "dependencies.zip")
	if err := writeArtifactExportArchive(repoPath, packages, "sh", outputPath); err != nil {
		t.Fatal(err)
	}

	archive, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()

	entries := make(map[string]string, len(archive.File))
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		contents, err := io.ReadAll(reader)
		if err != nil {
			_ = reader.Close()
			t.Fatal(err)
		}
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}
		entries[file.Name] = string(contents)
	}

	if entries["com/example/library/1.2.3/library-1.2.3.jar"] != "jar-binary" {
		t.Error("archive does not contain the JAR payload")
	}
	if entries["com/example/library/1.2.3/library-1.2.3.pom"] != "<project/>" {
		t.Error("archive does not contain the first POM")
	}
	if entries["org/sample/widget/2.0/widget-2.0.pom"] != "<project/>" {
		t.Error("archive does not contain the second POM")
	}
	script := entries["install-to-maven.sh"]
	if !strings.Contains(script, "maven-install-plugin:2.5.2:install-file") ||
		!strings.Contains(script, `-DpomFile="$SCRIPT_DIR/com/example/library/1.2.3/library-1.2.3.pom"`) ||
		!strings.Contains(script, `-DpomFile="$SCRIPT_DIR/org/sample/widget/2.0/widget-2.0.pom"`) {
		t.Error("shell script does not contain install commands for all selected versions")
	}
	if !strings.Contains(entries["README.txt"], "standard Maven local repository layout") ||
		!strings.Contains(entries["README.txt"], "extract the archive at that repository's root") {
		t.Error("archive README does not explain direct copying into a Maven repository")
	}
	if _, exists := entries["install-to-maven.bat"]; exists {
		t.Error("archive contains an unselected Windows script")
	}
}

func TestWriteArtifactExportArchiveRejectsInvalidInput(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "out.zip")
	if err := writeArtifactExportArchive(t.TempDir(), nil, "bat", outputPath); err == nil {
		t.Fatal("empty artifact list was accepted")
	}
	if err := writeArtifactExportArchive(t.TempDir(), []catalog.ArtifactCoordinate{
		{GroupID: "..", ArtifactID: "library", Version: "1.0"},
	}, "bat", outputPath); err == nil {
		t.Fatal("invalid group ID was accepted")
	}
	if err := writeArtifactExportArchive(t.TempDir(), []catalog.ArtifactCoordinate{
		{GroupID: "com.example", ArtifactID: "library", Version: "1.0"},
	}, "ps1", outputPath); err == nil {
		t.Fatal("unsupported script type was accepted")
	}
}

func TestWriteArtifactExportArchiveGeneratesWindowsScript(t *testing.T) {
	repoPath := t.TempDir()
	versionDir := filepath.Join(repoPath, "com", "example", "library", "1.0")
	if err := os.MkdirAll(versionDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "library-1.0.jar"), []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}
	outputPath := filepath.Join(t.TempDir(), "dependencies.zip")
	artifacts := []catalog.ArtifactCoordinate{
		{GroupID: "com.example", ArtifactID: "library", Version: "1.0"},
	}
	if err := writeArtifactExportArchive(repoPath, artifacts, "bat", outputPath); err != nil {
		t.Fatal(err)
	}

	archive, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 3 {
		t.Fatalf("archive entries = %d, want artifact, Windows script, and README", len(archive.File))
	}
	for _, file := range archive.File {
		if file.Name == "install-to-maven.bat" {
			reader, err := file.Open()
			if err != nil {
				t.Fatal(err)
			}
			contents, err := io.ReadAll(reader)
			_ = reader.Close()
			if err != nil {
				t.Fatal(err)
			}
			script := string(contents)
			if !strings.Contains(script, "call mvn org.apache.maven.plugins:maven-install-plugin:2.5.2:install-file") ||
				!strings.Contains(script, "-DgroupId=com.example") ||
				!strings.Contains(script, "-Dpackaging=jar") {
				t.Errorf("Windows script does not contain expected install command: %s", script)
			}
			return
		}
		if file.Name == "install-to-maven.sh" {
			t.Error("archive contains an unselected shell script")
		}
	}
	t.Fatal("Windows install script is missing")
}
