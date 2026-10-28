package catalog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPackaging(t *testing.T) {
	tests := []struct {
		name        string
		pom         string
		want        string
		wantErr     bool
		wantInvalid bool
	}{
		{
			name: "defaults to jar",
			pom:  `<project><modelVersion>4.0.0</modelVersion></project>`,
			want: "jar",
		},
		{
			name: "reads explicit packaging",
			pom:  `<project><packaging> pom </packaging></project>`,
			want: "pom",
		},
		{
			name: "reads namespaced pom",
			pom:  `<project xmlns="http://maven.apache.org/POM/4.0.0"><packaging>war</packaging></project>`,
			want: "war",
		},
		{
			name: "reads ISO-8859-1 encoded pom",
			pom:  "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><project><name>caf\xe9</name><packaging>jar</packaging></project>",
			want: "jar",
		},
		{
			name: "reads US-ASCII encoded pom",
			pom:  "<?xml version=\"1.0\" encoding=\"US-ASCII\"?><project><packaging>pom</packaging></project>",
			want: "pom",
		},
		{
			name: "reads packaging when unrelated metadata contains invalid entities",
			pom:  `<project><name>Legacy &_ project &oslash; &amp</name><packaging>jar</packaging></project>`,
			want: "jar",
		},
		{
			name:        "rejects malformed pom",
			pom:         `<project><packaging>jar</project>`,
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:        "rejects HTML error page",
			pom:         `<!DOCTYPE html><html><head><meta name="csrf_token"></head><body>Access denied</body></html>`,
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:        "rejects well-formed non-project XML",
			pom:         `<html><packaging>jar</packaging></html>`,
			wantErr:     true,
			wantInvalid: true,
		},
		{
			name:        "rejects unsupported XML encoding",
			pom:         `<?xml version="1.0" encoding="unknown-charset"?><project><packaging>jar</packaging></project>`,
			wantErr:     true,
			wantInvalid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pom.xml")
			if err := os.WriteFile(path, []byte(tt.pom), 0600); err != nil {
				t.Fatal(err)
			}

			got, err := ReadPackaging(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadPackaging() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantInvalid && !errors.Is(err, ErrInvalidPOM) {
				t.Errorf("ReadPackaging() error = %v, want ErrInvalidPOM", err)
			}
			if got != tt.want {
				t.Errorf("ReadPackaging() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsJarMissing(t *testing.T) {
	dir := t.TempDir()
	pomPath := filepath.Join(dir, "artifact-1.0.pom")
	if err := os.WriteFile(pomPath, []byte(`<project/>`), 0600); err != nil {
		t.Fatal(err)
	}

	missing, err := IsJarMissing(pomPath)
	if err != nil {
		t.Fatal(err)
	}
	if !missing {
		t.Fatal("IsJarMissing() = false, want true when the matching JAR is absent")
	}

	jarPath := filepath.Join(dir, "artifact-1.0.jar")
	if err := os.WriteFile(jarPath, []byte("jar"), 0600); err != nil {
		t.Fatal(err)
	}

	missing, err = IsJarMissing(pomPath)
	if err != nil {
		t.Fatal(err)
	}
	if missing {
		t.Fatal("IsJarMissing() = true, want false when the matching JAR exists")
	}
}
