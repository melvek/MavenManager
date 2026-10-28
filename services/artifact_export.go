package services

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"mvnmanager/internal/catalog"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func (s *RepoService) ExportArtifactVersions(
	repoPath string,
	artifacts []catalog.ArtifactCoordinate,
	scriptType string,
) (string, error) {
	if len(artifacts) == 0 {
		return "", fmt.Errorf("no Maven artifact versions selected for export")
	}
	if scriptType != "bat" && scriptType != "sh" {
		return "", fmt.Errorf("unsupported install script type %q", scriptType)
	}
	for _, artifact := range artifacts {
		if !validMavenCoordinate(artifact.GroupID, artifact.ArtifactID, artifact.Version) {
			return "", fmt.Errorf("invalid Maven coordinates: %s:%s:%s", artifact.GroupID, artifact.ArtifactID, artifact.Version)
		}
	}

	app := application.Get()
	if app == nil {
		return "", fmt.Errorf("Wails 应用尚未启动")
	}
	dialog := app.Dialog.SaveFile()
	dialog.SetOptions(&application.SaveFileDialogOptions{
		Title:    "导出 Maven 依赖包",
		Filename: "maven-dependencies.zip",
		Filters: []application.FileFilter{
			{DisplayName: "ZIP 压缩包", Pattern: "*.zip"},
		},
	})
	outputPath, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("选择导出位置失败: %w", err)
	}
	if outputPath == "" {
		return "", nil
	}
	if !strings.EqualFold(filepath.Ext(outputPath), ".zip") {
		outputPath += ".zip"
	}
	if err := writeArtifactExportArchive(repoPath, artifacts, scriptType, outputPath); err != nil {
		return "", err
	}
	return outputPath, nil
}

func writeArtifactExportArchive(
	repoPath string,
	artifacts []catalog.ArtifactCoordinate,
	scriptType string,
	outputPath string,
) error {
	if len(artifacts) == 0 {
		return fmt.Errorf("no Maven artifact versions selected for export")
	}
	if scriptType != "bat" && scriptType != "sh" {
		return fmt.Errorf("unsupported install script type %q", scriptType)
	}
	repoAbs, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("resolve Maven repository path: %w", err)
	}
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve export file path: %w", err)
	}
	tempFile, err := os.CreateTemp(filepath.Dir(outputAbs), ".mavenmanager-export-*.tmp")
	if err != nil {
		return fmt.Errorf("create export archive: %w", err)
	}
	tempPath := tempFile.Name()
	tempClosed := false
	defer func() {
		if !tempClosed {
			_ = tempFile.Close()
		}
		_ = os.Remove(tempPath)
	}()

	writer := zip.NewWriter(tempFile)
	installCommands := make([]string, 0, len(artifacts))
	seen := make(map[catalog.ArtifactCoordinate]struct{}, len(artifacts))
	for _, artifact := range artifacts {
		if !validMavenCoordinate(artifact.GroupID, artifact.ArtifactID, artifact.Version) {
			_ = writer.Close()
			return fmt.Errorf("invalid Maven coordinates: %s:%s:%s", artifact.GroupID, artifact.ArtifactID, artifact.Version)
		}
		if _, ok := seen[artifact]; ok {
			continue
		}
		seen[artifact] = struct{}{}
		command, err := addArtifactToExport(writer, repoAbs, artifact, outputAbs, tempPath, scriptType)
		if err != nil {
			_ = writer.Close()
			return err
		}
		installCommands = append(installCommands, command)
	}
	if len(installCommands) == 0 {
		_ = writer.Close()
		return fmt.Errorf("no Maven artifact versions selected for export")
	}

	scriptName := "install-to-maven." + scriptType
	script := buildInstallScript(scriptType, installCommands)
	if err := addZipFile(writer, scriptName, script); err != nil {
		_ = writer.Close()
		return err
	}
	if err := addZipFile(writer, "README.txt", exportReadme(scriptName)); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish export archive: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		tempClosed = true
		return fmt.Errorf("close export archive: %w", err)
	}
	tempClosed = true

	if err := os.Rename(tempPath, outputAbs); err != nil {
		if _, statErr := os.Stat(outputAbs); statErr == nil {
			if removeErr := os.Remove(outputAbs); removeErr != nil {
				return fmt.Errorf("replace export file: %w", removeErr)
			}
			if renameErr := os.Rename(tempPath, outputAbs); renameErr == nil {
				return nil
			} else {
				return fmt.Errorf("save export archive: %w", renameErr)
			}
		}
		return fmt.Errorf("save export archive: %w", err)
	}
	return nil
}

func exportReadme(scriptName string) string {
	return "This archive preserves the standard Maven local repository layout.\r\n" +
		"To copy these artifacts into another local repository, extract the archive at that repository's root.\r\n" +
		"For example, extract into ~/.m2/repository (or the configured local repository directory).\r\n\r\n" +
		"To import the selected artifacts with Maven instead, extract the archive anywhere and run " + scriptName + ".\r\n" +
		"The script runs Maven install:install-file for each selected artifact version.\r\n"
}

func addArtifactToExport(
	writer *zip.Writer,
	repoPath string,
	artifact catalog.ArtifactCoordinate,
	outputPath string,
	tempPath string,
	scriptType string,
) (string, error) {
	groupPath := filepath.Join(strings.Split(artifact.GroupID, ".")...)
	versionDir := filepath.Join(repoPath, groupPath, artifact.ArtifactID, artifact.Version)
	info, err := os.Stat(versionDir)
	if err != nil {
		return "", fmt.Errorf("access Maven version directory for %s:%s:%s: %w",
			artifact.GroupID, artifact.ArtifactID, artifact.Version, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Maven version path is not a directory: %s", versionDir)
	}
	repoRealPath, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		return "", fmt.Errorf("resolve Maven repository directory: %w", err)
	}
	versionRealPath, err := filepath.EvalSymlinks(versionDir)
	if err != nil {
		return "", fmt.Errorf("resolve Maven version directory: %w", err)
	}
	relativeRealPath, err := filepath.Rel(repoRealPath, versionRealPath)
	if err != nil || relativeRealPath == ".." ||
		strings.HasPrefix(relativeRealPath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Maven version directory is outside the configured repository")
	}
	versionDir = versionRealPath

	relativeDir := path.Join(strings.Split(artifact.GroupID, ".")...)
	relativeDir = path.Join(relativeDir, artifact.ArtifactID, artifact.Version)
	baseName := artifact.ArtifactID + "-" + artifact.Version
	pomFile := ""
	primaryArtifact := ""
	err = filepath.WalkDir(versionDir, func(filePath string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		absolutePath, err := filepath.Abs(filePath)
		if err != nil {
			return err
		}
		if filepath.Clean(absolutePath) == filepath.Clean(outputPath) ||
			filepath.Clean(absolutePath) == filepath.Clean(tempPath) {
			return nil
		}
		relativePath, err := filepath.Rel(versionDir, filePath)
		if err != nil {
			return err
		}
		archivePath := path.Join(relativeDir, filepath.ToSlash(relativePath))
		fileInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("read file information for %s: %w", filePath, err)
		}
		header, err := zip.FileInfoHeader(fileInfo)
		if err != nil {
			return err
		}
		header.Name = archivePath
		header.Method = zip.Deflate
		fileWriter, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		source, err := os.Open(filePath)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(fileWriter, source)
		closeErr := source.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}

		fileName := filepath.Base(filePath)
		switch fileName {
		case baseName + ".pom":
			pomFile = filepath.ToSlash(path.Join(relativeDir, baseName+".pom"))
		case baseName + ".jar":
			primaryArtifact = filepath.ToSlash(path.Join(relativeDir, filepath.Base(filePath)))
		case baseName + ".war", baseName + ".ear", baseName + ".zip":
			if primaryArtifact == "" {
				primaryArtifact = filepath.ToSlash(path.Join(relativeDir, filepath.Base(filePath)))
			}
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("add %s:%s:%s to export: %w",
			artifact.GroupID, artifact.ArtifactID, artifact.Version, err)
	}
	if pomFile == "" && primaryArtifact == "" {
		return "", fmt.Errorf("%s:%s:%s contains no POM or primary artifact",
			artifact.GroupID, artifact.ArtifactID, artifact.Version)
	}
	return buildInstallCommand(scriptType, artifact, primaryArtifact, pomFile), nil
}

func buildInstallCommand(
	scriptType string,
	artifact catalog.ArtifactCoordinate,
	primaryArtifact string,
	pomFile string,
) string {
	var args []string
	artifactPath := primaryArtifact
	if artifactPath == "" {
		artifactPath = pomFile
	}
	if scriptType == "bat" {
		args = append(args, `-Dfile="%~dp0`+strings.ReplaceAll(artifactPath, "/", `\`)+`"`)
		if pomFile != "" && primaryArtifact != "" {
			args = append(args, `-DpomFile="%~dp0`+strings.ReplaceAll(pomFile, "/", `\`)+`"`)
		} else if primaryArtifact == "" {
			args = append(args, `-DpomFile="%~dp0`+strings.ReplaceAll(pomFile, "/", `\`)+`"`, "-Dpackaging=pom")
		} else {
			args = append(args,
				"-DgroupId="+artifact.GroupID,
				"-DartifactId="+artifact.ArtifactID,
				"-Dversion="+artifact.Version,
				"-Dpackaging="+filepath.Ext(primaryArtifact)[1:],
			)
		}
		return "call mvn org.apache.maven.plugins:maven-install-plugin:2.5.2:install-file " + strings.Join(args, " ")
	}

	args = append(args, `-Dfile="$SCRIPT_DIR/`+artifactPath+`"`)
	if pomFile != "" && primaryArtifact != "" {
		args = append(args, `-DpomFile="$SCRIPT_DIR/`+pomFile+`"`)
	} else if primaryArtifact == "" {
		args = append(args, `-DpomFile="$SCRIPT_DIR/`+pomFile+`"`, "-Dpackaging=pom")
	} else {
		args = append(args,
			"-DgroupId="+artifact.GroupID,
			"-DartifactId="+artifact.ArtifactID,
			"-Dversion="+artifact.Version,
			"-Dpackaging="+filepath.Ext(primaryArtifact)[1:],
		)
	}
	return "mvn org.apache.maven.plugins:maven-install-plugin:2.5.2:install-file " + strings.Join(args, " ")
}

func buildInstallScript(scriptType string, commands []string) string {
	var script strings.Builder
	if scriptType == "bat" {
		script.WriteString("@echo off\r\nsetlocal\r\n")
		for _, command := range commands {
			script.WriteString(command + "\r\n")
			script.WriteString("if errorlevel 1 exit /b %errorlevel%\r\n")
		}
		script.WriteString("echo All selected dependencies were installed into the local Maven repository.\r\n")
		return script.String()
	}
	script.WriteString("#!/bin/sh\nset -eu\nSCRIPT_DIR=$(CDPATH= cd -- \"$(dirname -- \"$0\")\" && pwd)\n")
	for _, command := range commands {
		script.WriteString(command + "\n")
	}
	return script.String()
}

func addZipFile(writer *zip.Writer, name, contents string) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	if strings.HasSuffix(name, ".sh") {
		header.SetMode(0755)
	}
	file, err := writer.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("add %s to export archive: %w", name, err)
	}
	if _, err := io.WriteString(file, contents); err != nil {
		return fmt.Errorf("write %s to export archive: %w", name, err)
	}
	return nil
}
