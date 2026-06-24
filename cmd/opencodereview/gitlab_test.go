package main

import (
	"os/exec"
	"testing"
)

func TestParseGitLabBaseURL(t *testing.T) {
	tests := []struct {
		remote string
		want   string
	}{
		{"https://gitlab.com/group/project.git", "https://gitlab.com"},
		{"https://gitlab.example.com:8443/group/project.git", "https://gitlab.example.com:8443"},
		{"http://gitlab.internal/group/project.git", "http://gitlab.internal"},
		{"git@gitlab.com:group/project.git", "https://gitlab.com"},
		{"ssh://git@git.example.com:4422/company/project-next.git", "https://git.example.com"},
        {"ssh://git@gitlab.com/group/project.git", "https://gitlab.com"},
		{"", ""},
		{"not-a-remote", ""},
	}

	for _, tc := range tests {
		got := parseGitLabBaseURL(tc.remote)
		if got != tc.want {
			t.Errorf("parseGitLabBaseURL(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

func TestParseProjectPathFromRemote(t *testing.T) {
	tests := []struct {
		remote  string
		want    string
		wantErr bool
	}{
		{"https://gitlab.com/my-group/my-project.git", "my-group/my-project", false},
		{"git@gitlab.com:my-group/my-project.git", "my-group/my-project", false},
		{"ssh://git@gitlab.com/my-group/my-project.git", "my-group/my-project", false},
		{"", "", true},
		{"unsupported", "", true},
	}

	for _, tc := range tests {
		got, err := parseProjectPathFromRemote(tc.remote)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parseProjectPathFromRemote(%q): expected error", tc.remote)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseProjectPathFromRemote(%q): %v", tc.remote, err)
		}
		if got != tc.want {
			t.Errorf("parseProjectPathFromRemote(%q) = %q, want %q", tc.remote, got, tc.want)
		}
	}
}

func TestDetectProjectIDAndGitLabURLFromRepo(t *testing.T) {
	dir := initGitRepoWithRemote(t, "https://gitlab.com/acme/widget.git")

	gotID, err := detectProjectID(dir)
	if err != nil {
		t.Fatalf("detectProjectID: %v", err)
	}
	if gotID != "acme/widget" {
		t.Errorf("detectProjectID = %q, want %q", gotID, "acme/widget")
	}

	gotURL := detectGitLabURL(dir)
	if gotURL != "https://gitlab.com" {
		t.Errorf("detectGitLabURL = %q, want %q", gotURL, "https://gitlab.com")
	}
}

func TestDetectGitLabURL_StripsSSHGitPort(t *testing.T) {
	dir := initGitRepoWithRemote(t, "ssh://git@git.valiton.com:22022/techmuc/modash/modash-next.git")

	gotURL := detectGitLabURL(dir)
	if gotURL != "https://git.valiton.com" {
		t.Errorf("detectGitLabURL = %q, want %q", gotURL, "https://git.valiton.com")
	}
}

func initGitRepoWithRemote(t *testing.T, remoteURL string) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	runGit("remote", "add", "origin", remoteURL)
	return dir
}
