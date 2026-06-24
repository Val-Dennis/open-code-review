package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/open-code-review/open-code-review/internal/config/template"
)

const (
	repoConfigDir      = ".opencodereview"
	repoConfigName     = "config.json"
	repoConfigRelPath  = ".opencodereview/config.json"
	repoConfigGitEntry = ".opencodereview/config.json"
)

// RepoConfig holds the per-repository configuration from .opencodereview/config.json.
type RepoConfig struct {
	Version string                  `json:"version,omitempty"`
	GitLab  *GitLabSection          `json:"gitlab,omitempty"`
	LLM     *LLMSection             `json:"llm,omitempty"`
	Review  *ReviewSection          `json:"review,omitempty"`
	Prompts *template.PromptsConfig `json:"prompts,omitempty"`
}

// GitLabSection holds GitLab-specific config in .opencodereview/config.json.
type GitLabSection struct {
	URL       string `json:"url,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Token     string `json:"token,omitempty"`
}

// LLMSection holds LLM overrides in .opencodereview/config.json.
type LLMSection struct {
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	URL        string `json:"url,omitempty"`
	AuthHeader string `json:"auth_header,omitempty"`
}

// ReviewSection holds review defaults in .opencodereview/config.json.
type ReviewSection struct {
	AutoPost   bool   `json:"auto_post,omitempty"`
	MaxTools   int    `json:"max_tools,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Background string `json:"background,omitempty"`
}

// repoConfigPath returns the path to .opencodereview/config.json in the repo root.
func repoConfigPath(repoDir string) string {
	return filepath.Join(repoDir, repoConfigDir, repoConfigName)
}

// gitignorePath returns the path to .gitignore in the repo root.
func gitignorePath(repoDir string) string {
	return filepath.Join(repoDir, ".gitignore")
}

// loadRepoConfig reads .opencodereview/config.json from the repo root.
// Returns an empty config if the file doesn't exist.
func loadRepoConfig(repoDir string) (*RepoConfig, error) {
	path := repoConfigPath(repoDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &RepoConfig{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", repoConfigRelPath, err)
	}

	var cfg RepoConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", repoConfigRelPath, err)
	}
	return &cfg, nil
}

// saveRepoConfig writes .opencodereview/config.json to the repo root.
func saveRepoConfig(repoDir string, cfg *RepoConfig) error {
	path := repoConfigPath(repoDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create %s: %w", repoConfigDir, err)
	}
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write %s: %w", repoConfigRelPath, err)
	}
	return nil
}

// ensureRepoConfigGitignore ensures .opencodereview/config.json is in .gitignore.
func ensureRepoConfigGitignore(repoDir string) error {
	path := gitignorePath(repoDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(path, []byte(repoConfigGitEntry+"\n"), 0644)
		}
		return fmt.Errorf("read .gitignore: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == repoConfigGitEntry {
			return nil
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("append .gitignore: %w", err)
	}
	defer f.Close()

	if len(data) > 0 && data[len(data)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}
	_, err = f.WriteString(repoConfigGitEntry + "\n")
	return err
}
