package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvSetsMissingValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("DATABASE_URL=postgres://example\nNVIDIA_MODEL='google/gemma-4-31b-it'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", "")
	t.Setenv("NVIDIA_MODEL", "")

	LoadDotEnv(path)
	cfg := Load()

	if cfg.DatabaseURL != "postgres://example" {
		t.Fatalf("database url = %q", cfg.DatabaseURL)
	}
	if cfg.NVIDIAModel != "google/gemma-4-31b-it" {
		t.Fatalf("nvidia model = %q", cfg.NVIDIAModel)
	}
}
