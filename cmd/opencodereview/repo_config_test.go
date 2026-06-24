package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadRepoConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &RepoConfig{
		GitLab: &GitLabSection{
			URL:       "https://gitlab.example.com:8443",
			ProjectID: "acme/widget",
			Token:     "glpat-test",
		},
		Review: &ReviewSection{
			AutoPost: true,
			MaxTools: 15,
			From:     "origin/main",
		},
	}

	if err := saveRepoConfig(dir, cfg); err != nil {
		t.Fatalf("saveRepoConfig: %v", err)
	}

	path := repoConfigPath(dir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("config mode = %o, want 0600", info.Mode().Perm())
	}

	loaded, err := loadRepoConfig(dir)
	if err != nil {
		t.Fatalf("loadRepoConfig: %v", err)
	}
	if loaded.GitLab == nil || loaded.GitLab.Token != "glpat-test" {
		t.Fatalf("loaded GitLab config = %#v", loaded.GitLab)
	}
	if loaded.GitLab.URL != "https://gitlab.example.com:8443" {
		t.Errorf("URL = %q, want %q", loaded.GitLab.URL, "https://gitlab.example.com:8443")
	}
	if loaded.Review == nil || !loaded.Review.AutoPost || loaded.Review.MaxTools != 15 {
		t.Fatalf("loaded Review config = %#v", loaded.Review)
	}
}

func TestLoadRepoConfigMissingFile(t *testing.T) {
	cfg, err := loadRepoConfig(t.TempDir())
	if err != nil {
		t.Fatalf("loadRepoConfig: %v", err)
	}
	if cfg.GitLab != nil || cfg.Review != nil {
		t.Fatalf("expected empty config, got %#v", cfg)
	}
}

func TestEnsureRepoConfigGitignore(t *testing.T) {
	dir := t.TempDir()

	if err := ensureRepoConfigGitignore(dir); err != nil {
		t.Fatalf("ensureRepoConfigGitignore: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), repoConfigGitEntry) {
		t.Errorf(".gitignore = %q, want entry %q", string(data), repoConfigGitEntry)
	}

	if err := ensureRepoConfigGitignore(dir); err != nil {
		t.Fatalf("ensureRepoConfigGitignore second call: %v", err)
	}
	data2, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if strings.Count(string(data2), repoConfigGitEntry) != 1 {
		t.Errorf(".gitignore should contain one entry, got %q", string(data2))
	}
}
