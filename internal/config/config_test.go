package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default()
	if d.DefaultView != "auto" || d.Indent != "  " || d.Theme != "default" {
		t.Errorf("unexpected defaults: %+v", d)
	}
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, _, err := Load(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.Theme != "default" {
		t.Errorf("expected defaults for missing file, got %+v", cfg)
	}
}

func TestLoadMergesOverDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Only set theme; indent and view should remain at defaults.
	if err := os.WriteFile(path, []byte(`{"theme":"colorblind"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "colorblind" {
		t.Errorf("expected theme colorblind, got %q", cfg.Theme)
	}
	if cfg.Indent != "  " || cfg.DefaultView != "auto" {
		t.Errorf("absent fields should keep defaults, got %+v", cfg)
	}
}

func TestLoadMalformedReturnsDefaultsAndError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{not valid json`), 0o644)
	cfg, _, err := Load(path)
	if err == nil {
		t.Error("expected an error for malformed config")
	}
	if cfg.Theme != "default" {
		t.Errorf("malformed config should fall back to defaults, got %+v", cfg)
	}
}

func TestNormalizeRepairsBadEnums(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	os.WriteFile(path, []byte(`{"theme":"rainbow","defaultView":"grid","indent":""}`), 0o644)
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Theme != "default" {
		t.Errorf("invalid theme should reset to default, got %q", cfg.Theme)
	}
	if cfg.DefaultView != "auto" {
		t.Errorf("invalid view should reset to auto, got %q", cfg.DefaultView)
	}
	if cfg.Indent != "  " {
		t.Errorf("empty indent should reset to two spaces, got %q", cfg.Indent)
	}
}

func TestNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	cfg, _, _ := Load(filepath.Join(t.TempDir(), "none.json"))
	if !cfg.NoColor {
		t.Error("NO_COLOR should set cfg.NoColor")
	}
}

func TestEnvConfigPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.json")
	os.WriteFile(path, []byte(`{"theme":"mono"}`), 0o644)
	t.Setenv("NANOJ_CONFIG", path)
	cfg, used, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if used != path {
		t.Errorf("expected to consult %s, used %s", path, used)
	}
	if cfg.Theme != "mono" {
		t.Errorf("expected theme mono from env path, got %q", cfg.Theme)
	}
}

func TestWriteExampleIsValidAndReloadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "config.json")
	written, err := WriteExample(path)
	if err != nil {
		t.Fatal(err)
	}
	if written != path {
		t.Errorf("expected %s, got %s", path, written)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatalf("written example should reload cleanly: %v", err)
	}
	if cfg.Theme != "default" || len(cfg.Styles) == 0 {
		t.Errorf("example should include defaults and sample styles, got %+v", cfg)
	}
}
