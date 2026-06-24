package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/open-code-review/open-code-review/internal/agent"
	"github.com/open-code-review/open-code-review/internal/model"
)

var scpStyleSSHHost = regexp.MustCompile(`^git@([^:/]+)`)

var gitlabHTTPClient = &http.Client{Timeout: 30 * time.Second}

// gitlabClient handles GitLab API operations for a specific project.
type gitlabClient struct {
	BaseURL    string
	Token      string
	ProjectID  string // numeric ID or URL-encoded path (e.g. "group%2Fproject")
	MRIIID     int
	httpClient *http.Client
}

type gitlabVersion struct {
	BaseCommitSHA  string `json:"base_commit_sha"`
	StartCommitSHA string `json:"start_commit_sha"`
	HeadCommitSHA  string `json:"head_commit_sha"`
}

type gitlabMR struct {
	IID int `json:"iid"`
}

type gitlabAccessToken struct {
	Token   string   `json:"token"`
	ID      int      `json:"id"`
	UserID  int      `json:"user_id"`
	Scopes  []string `json:"scopes"`
	Name    string   `json:"name"`
	Expires string   `json:"expires_at"`
}

// newGitLabClient creates a client using the project-scoped token from .opencodereview/config.json.
func newGitLabClient(token, projectID, baseURL string) *gitlabClient {
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	return &gitlabClient{
		BaseURL:    baseURL,
		Token:      token,
		ProjectID:  url.PathEscape(projectID),
		httpClient: gitlabHTTPClient,
	}
}

// parseGitLabBaseURL extracts the GitLab instance base URL from a git remote URL.
// HTTP(S) remotes keep host and port because that is the API endpoint.
// SSH remotes drop the port.
func parseGitLabBaseURL(remote string) string {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return ""
	}
	if strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "ssh://") || strings.HasPrefix(remote, "http://") {
		u, err := url.Parse(remote)
		if err != nil || u.Host == "" {
			return ""
		}
		scheme := "https"
		if u.Scheme == "http" {
			scheme = "http"
		}
		host := u.Host
		if u.Scheme == "ssh" {
			host = u.Hostname()
		}
		return fmt.Sprintf("%s://%s", scheme, host)
	}
	if m := scpStyleSSHHost.FindStringSubmatch(remote); len(m) > 1 {
		return fmt.Sprintf("https://%s", m[1])
	}
	return ""
}

// detectGitLabURL extracts the GitLab instance URL from the git remote.
func detectGitLabURL(repoDir string) string {
	out, err := runGitCmd(repoDir, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return parseGitLabBaseURL(string(out))
}

// parseProjectPathFromRemote extracts the project path (group/project) from a git remote URL.
func parseProjectPathFromRemote(remote string) (string, error) {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return "", fmt.Errorf("no remote 'origin' configured")
	}

	var projectPath string
	if strings.HasPrefix(remote, "https://") || strings.HasPrefix(remote, "ssh://") || strings.HasPrefix(remote, "http://") {
		u, err := url.Parse(remote)
		if err != nil {
			return "", fmt.Errorf("parse remote URL: %w", err)
		}
		projectPath = strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git")
	} else if strings.HasPrefix(remote, "git@") {
		parts := strings.SplitN(remote, ":", 2)
		if len(parts) != 2 {
			return "", fmt.Errorf("cannot parse SSH remote: %s", remote)
		}
		projectPath = strings.TrimSuffix(parts[1], ".git")
	} else {
		return "", fmt.Errorf("unsupported remote format: %s", remote)
	}
	if projectPath == "" {
		return "", fmt.Errorf("could not extract project path from remote: %s", remote)
	}
	return projectPath, nil
}

// detectProjectID extracts the project path (group/project) from git remote URL.
func detectProjectID(repoDir string) (string, error) {
	out, err := runGitCmd(repoDir, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("get remote origin: %w", err)
	}
	return parseProjectPathFromRemote(string(out))
}

// findMRByBranch searches for an open MR matching the given branch.
func (c *gitlabClient) findMRByBranch(branch string) (int, error) {
	u := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests?source_branch=%s&state=opened&per_page=5",
		c.BaseURL, c.ProjectID, url.QueryEscape(branch))

	resp, err := c.doRequest("GET", u, nil)
	if err != nil {
		return 0, fmt.Errorf("search MRs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GitLab API error %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var mrs []gitlabMR
	if err := json.NewDecoder(resp.Body).Decode(&mrs); err != nil {
		return 0, fmt.Errorf("parse MR response: %w", err)
	}
	if len(mrs) == 0 {
		return 0, fmt.Errorf("no open MR found for branch '%s'", branch)
	}
	return mrs[0].IID, nil
}

// autoDetectMR finds an open MR for the current git branch and sets c.MRIIID.
func (c *gitlabClient) autoDetectMR(repoDir string) error {
	if v := os.Getenv("CI_MERGE_REQUEST_IID"); v != "" {
		iid, err := strconv.Atoi(v)
		if err == nil && iid > 0 {
			c.MRIIID = iid
			fmt.Fprintf(os.Stderr, "[ocr] Using MR !%d from CI_MERGE_REQUEST_IID\n", c.MRIIID)
			return nil
		}
	}

	out, err := runGitCmd(repoDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("get current branch: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" || branch == "HEAD" {
		return fmt.Errorf("not on a named branch (detached HEAD)")
	}

	iid, err := c.findMRByBranch(branch)
	if err != nil {
		return err
	}
	c.MRIIID = iid
	fmt.Fprintf(os.Stderr, "[ocr] Found MR !%d for branch '%s'\n", iid, branch)
	return nil
}

// fetchDiffRefs gets the diff versions for the MR for position calculation.
func (c *gitlabClient) fetchDiffRefs() (*gitlabVersion, error) {
	u := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/versions",
		c.BaseURL, c.ProjectID, c.MRIIID)

	resp, err := c.doRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("GET versions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET versions: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var versions []gitlabVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("parse versions: %w", err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for MR !%d", c.MRIIID)
	}
	v := versions[0]
	return &gitlabVersion{
		BaseCommitSHA:  v.BaseCommitSHA,
		StartCommitSHA: v.StartCommitSHA,
		HeadCommitSHA:  v.HeadCommitSHA,
	}, nil
}

// postDiscussion posts an inline discussion on the MR.
func (c *gitlabClient) postDiscussion(filePath string, line int, body string, refs *gitlabVersion) error {
	position := map[string]any{
		"position_type": "text",
		"new_path":      filePath,
		"old_path":      filePath,
		"new_line":      line,
		"base_sha":      refs.BaseCommitSHA,
		"start_sha":     refs.StartCommitSHA,
		"head_sha":      refs.HeadCommitSHA,
	}
	payload := map[string]any{
		"body":     body,
		"position": position,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discussion payload: %w", err)
	}

	u := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/discussions",
		c.BaseURL, c.ProjectID, c.MRIIID)

	resp, err := c.doRequest("POST", u, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("POST discussion: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// postNote posts a general note on the MR (not inline).
func (c *gitlabClient) postNote(body string) error {
	payload := map[string]string{"body": body}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal note payload: %w", err)
	}

	u := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/notes",
		c.BaseURL, c.ProjectID, c.MRIIID)

	resp, err := c.doRequest("POST", u, strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("POST note: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

// PostComments posts all review comments as inline discussions + summary on the MR.
func (c *gitlabClient) PostComments(comments []model.LlmComment, warnings []agent.AgentWarning) error {
	if len(comments) == 0 {
		return c.postNote("✅ **OpenCodeReview**: No issues found. Looks good to me.")
	}

	diffRefs, err := c.fetchDiffRefs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ocr] Warning: could not fetch MR versions: %v (comments will use fallback)\n", err)
	}

	successCount := 0
	var failedComments []model.LlmComment

	for _, cm := range comments {
		if cm.Path == "" || cm.EndLine == 0 || diffRefs == nil {
			failedComments = append(failedComments, cm)
			continue
		}

		body := cm.Content
		if cm.SuggestionCode != "" && cm.ExistingCode != "" {
			body += "\n\n**Suggestion:**\n"
			body += fmt.Sprintf("```suggestion:-0+0\n%s\n```", cm.SuggestionCode)
		}

		if err := c.postDiscussion(cm.Path, cm.EndLine, body, diffRefs); err != nil {
			fmt.Fprintf(os.Stderr, "[ocr] Failed to post comment for %s: %v\n", cm.Path, err)
			failedComments = append(failedComments, cm)
			time.Sleep(500 * time.Millisecond)
		} else {
			successCount++
			time.Sleep(1500 * time.Millisecond)
		}
	}

	fmt.Fprintf(os.Stderr, "[ocr] Posted %d/%d inline comments to MR !%d\n", successCount, len(comments), c.MRIIID)

	if len(failedComments) > 0 {
		var parts []string
		for _, cm := range failedComments {
			loc := cm.Path
			if cm.StartLine > 0 || cm.EndLine > 0 {
				loc = fmt.Sprintf("%s (L%d-L%d)", cm.Path, cm.StartLine, cm.EndLine)
			}
			md := fmt.Sprintf("### 📄 `%s`\n\n%s", loc, cm.Content)
			if cm.SuggestionCode != "" && cm.ExistingCode != "" {
				md += "\n\n<details><summary>💡 Suggested Change</summary>\n\n"
				md += fmt.Sprintf("**Before:**\n```\n%s\n```\n\n", cm.ExistingCode)
				md += fmt.Sprintf("**After:**\n```\n%s\n```\n\n", cm.SuggestionCode)
				md += "</details>"
			}
			parts = append(parts, md)
		}
		fallbackBody := "🔍 **OpenCodeReview** could not post inline for some issues:\n\n---\n\n" + strings.Join(parts, "\n\n---\n\n")
		if err := c.postNote(fallbackBody); err != nil {
			fmt.Fprintf(os.Stderr, "[ocr] Failed to post fallback summary: %v\n", err)
		}
	}

	summary := fmt.Sprintf("🔍 **OpenCodeReview** found **%d** issue(s).\n- ✅ %d inline comment(s)\n- 📝 %d posted as summary (missing line info)",
		len(comments), successCount, len(failedComments))
	if len(warnings) > 0 {
		summary += fmt.Sprintf("\n\n⚠️ %d warning(s) during review.", len(warnings))
	}
	return c.postNote(summary)
}

// createProjectToken creates a project-scoped access token using the personal token.
func createProjectToken(baseURL, projectID, personalToken string) (string, error) {
	if baseURL == "" {
		baseURL = "https://gitlab.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	payload := map[string]any{
		"name":   "OpenCodeReview",
		"scopes": []string{"api"},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal token payload: %w", err)
	}

	u := fmt.Sprintf("%s/api/v4/projects/%s/access_tokens",
		baseURL, url.PathEscape(projectID))

	req, err := http.NewRequest("POST", u, strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("PRIVATE-TOKEN", personalToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := gitlabHTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create token request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("create token: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var token gitlabAccessToken
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if token.Token == "" {
		return "", fmt.Errorf("token response was empty")
	}

	return token.Token, nil
}

func (c *gitlabClient) doRequest(method, reqURL string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, reqURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("PRIVATE-TOKEN", c.Token)
	req.Header.Set("Content-Type", "application/json")
	return c.httpClient.Do(req)
}
