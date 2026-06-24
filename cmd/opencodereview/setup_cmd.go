package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/open-code-review/open-code-review/internal/stdout"
)

func printSetupUsage() {
	fmt.Fprint(os.Stderr, `Usage: ocr setup [--repo <dir>] [--no-pat]

Configure GitLab integration for the current repository.

Writes .opencodereview/config.json (adds it to .gitignore) with:
  - GitLab URL (auto-detected from git remote)
  - Project ID (auto-detected from git remote)
  - GitLab project token (auto-provisioned via PAT or prompted)
  - Review defaults (auto-post, max tools, diff source branch)

Flags:
  --repo string   root directory of the git repository (default: current dir)
  --no-pat        skip PAT auto-provision; prompt for token directly

Examples:
  ocr setup                         Auto-detect + PAT auto-provision or manual token
  ocr setup --no-pat                Manual token entry only
  ocr setup --repo /path/to/repo    Configure a specific repository
`)
}

// runSetup configures GitLab integration for the current repo.
func runSetup(args []string) error {
	a := newOcrFlagSet("ocr setup")
	var repoDir string
	var noPat bool
	a.StringVar(&repoDir, "repo", "", "root directory of the git repository (default: current dir)")
	a.BoolVar(&noPat, "no-pat", false, "skip PAT auto-provision; prompt for token directly")

	if err := a.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if a.showHelp {
		printSetupUsage()
		return nil
	}

	repoDir, err := resolveRepoDir(repoDir)
	if err != nil {
		return err
	}

	if err := requireGitRepo(repoDir); err != nil {
		return err
	}

	projectID, err := detectProjectID(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ocr] Could not detect project: %v\n", err)
		fmt.Fprintf(os.Stderr, "[ocr] You can set it later in %s\n", repoConfigRelPath)
		projectID = promptInput("GitLab project ID (e.g., 42 or my-namespace/my-project): ")
		if projectID == "" {
			return fmt.Errorf("project ID is required")
		}
	} else {
		fmt.Fprintf(stdout.Writer(), "Detected project: %s\n", projectID)
	}

	gitlabURL := detectGitLabURL(repoDir)
	if gitlabURL != "" {
		fmt.Fprintf(stdout.Writer(), "Detected GitLab URL: %s\n", gitlabURL)
	}

	token := ""
	if !noPat {
		token = tryAutoProvision(gitlabURL, projectID)
	}

	if token == "" {
		if noPat {
			fmt.Fprintln(os.Stderr, "\nPaste a GitLab project-scoped token.")
		} else {
			fmt.Fprintln(os.Stderr, "\nNo PAT configured or auto-provision failed.")
			fmt.Fprintln(os.Stderr, "Set one with: ocr config set gitlab.personal_token <token>")
			fmt.Fprintln(os.Stderr, "Or use 'ocr setup --no-pat' to skip PAT-based setup.")
		}
		fmt.Fprintln(os.Stderr, "Create one at: Settings → Access Tokens → Add new token (scope: api)")
		token = promptInput("GitLab project token: ")
		if token == "" {
			return fmt.Errorf("token is required")
		}
	}

	fmt.Fprintln(os.Stderr, "\nReview defaults (press Enter to skip):")
	autoPost := promptBool("Auto-post after review? [Y/n]: ", true)
	maxTools := promptInt("Max tool calls per file [15]: ", 15)
	fromBranch := promptInput("Default diff source branch [origin/main]: ")
	if fromBranch == "" {
		fromBranch = "origin/main"
	}

	cfg := &RepoConfig{
		GitLab: &GitLabSection{
			URL:       gitlabURL,
			ProjectID: projectID,
			Token:     token,
		},
		Review: &ReviewSection{
			AutoPost: autoPost,
			MaxTools: maxTools,
			From:     fromBranch,
		},
	}

	if err := saveRepoConfig(repoDir, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Fprintf(stdout.Writer(), "Written %s\n", repoConfigRelPath)

	if err := ensureRepoConfigGitignore(repoDir); err != nil {
		fmt.Fprintf(os.Stderr, "[ocr] Warning: could not update .gitignore: %v\n", err)
	} else {
		fmt.Fprintf(stdout.Writer(), "Added %s to .gitignore\n", repoConfigGitEntry)
	}

	fmt.Fprintln(os.Stderr, "\n✅ Setup complete. Run 'ocr review' to review this repo.")
	return nil
}

func tryAutoProvision(gitlabURL, projectID string) string {
	cfgPath, err := defaultConfigPath()
	if err != nil {
		return ""
	}
	cfg, err := LoadAppConfig(cfgPath)
	if err != nil || cfg == nil {
		return ""
	}
	if cfg.GitLabToken == "" {
		return ""
	}

	token, err := createProjectToken(gitlabURL, projectID, cfg.GitLabToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ocr] Auto-provision failed: %v\n", err)
		return ""
	}

	fmt.Fprintf(os.Stderr, "[ocr] Created project-scoped token\n")
	return token
}

func promptInput(prompt string) string {
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		return strings.TrimSpace(scanner.Text())
	}
	return ""
}

func promptBool(prompt string, defaultVal bool) bool {
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		val := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if val == "y" || val == "yes" {
			return true
		}
		if val == "n" || val == "no" {
			return false
		}
	}
	return defaultVal
}

func promptInt(prompt string, defaultVal int) int {
	fmt.Fprint(os.Stderr, prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		if val != "" {
			var n int
			if _, err := fmt.Sscanf(val, "%d", &n); err == nil {
				return n
			}
		}
	}
	return defaultVal
}
