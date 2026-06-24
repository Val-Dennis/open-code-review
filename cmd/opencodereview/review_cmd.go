package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/open-code-review/open-code-review/internal/agent"
	"github.com/open-code-review/open-code-review/internal/config/rules"
	"github.com/open-code-review/open-code-review/internal/config/template"
	"github.com/open-code-review/open-code-review/internal/config/toolsconfig"
	"github.com/open-code-review/open-code-review/internal/diff"
	"github.com/open-code-review/open-code-review/internal/gitcmd"
	"github.com/open-code-review/open-code-review/internal/llm"
	"github.com/open-code-review/open-code-review/internal/model"
	"github.com/open-code-review/open-code-review/internal/stdout"
	"github.com/open-code-review/open-code-review/internal/telemetry"
	"github.com/open-code-review/open-code-review/internal/tool"
)

func runReview(args []string) error {
	opts, err := parseReviewFlags(args)
	if err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}
	if opts.showHelp {
		printReviewUsage()
		return nil
	}

	if err := requireGitRepo(opts.repoDir); err != nil {
		return err
	}

	tpl, err := template.LoadDefault()
	if err != nil {
		return fmt.Errorf("load default template: %w", err)
	}
	if opts.maxTools > 0 {
		tpl.MaxToolRequestTimes = opts.maxTools
	}
	if err := tpl.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	repoDir, err := resolveRepoDir(opts.repoDir)
	if err != nil {
		return fmt.Errorf("resolve repo: %w", err)
	}

	// Load per-repo config and merge defaults
	repoCfg, err := loadRepoConfig(repoDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[ocr] Warning: %v\n", err)
	}
	if repoCfg != nil && repoCfg.Review != nil {
		rc := repoCfg.Review
		if opts.maxTools <= 0 && rc.MaxTools > 0 {
			opts.maxTools = rc.MaxTools
		}
		if opts.from == "" && rc.From != "" {
			opts.from = rc.From
		}
		if opts.background == "" && rc.Background != "" {
			opts.background = rc.Background
		}
	}
	if repoCfg != nil && repoCfg.LLM != nil {
		llm := repoCfg.LLM
		if opts.model == "" && llm.Model != "" {
			opts.model = llm.Model
		}
	}

	if err := validateReviewRefs(repoDir, opts); err != nil {
		return err
	}

	if opts.commit != "" && opts.background == "" {
		if msg, err := getCommitMessage(repoDir, opts.commit); err == nil && msg != "" {
			opts.background = msg
		}
	}

	resolver, fileFilter, err := rules.NewResolver(repoDir, opts.rulePath)
	if err != nil {
		return fmt.Errorf("load rules: %w", err)
	}

	if opts.preview {
		return runPreview(repoDir, opts, fileFilter)
	}

	toolEntries, err := toolsconfig.Load(opts.toolConfigPath)
	if err != nil {
		return fmt.Errorf("load tools: %w", err)
	}
	planToolDefs := agent.BuildToolDefs(toolEntries, true)
	mainToolDefs := agent.BuildToolDefs(toolEntries, false)

	cfgPath, err := defaultConfigPath()
	if err != nil {
		return err
	}

	appCfg, err := LoadAppConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("load app config: %w", err)
	}
	var lang string
	if appCfg != nil {
		lang = appCfg.Language
	}
	tpl.ApplyLanguage(lang)

	ep, err := llm.ResolveEndpointWithModelOverride(cfgPath, opts.model)
	if err != nil {
		return fmt.Errorf("resolve LLM endpoint: %w", err)
	}

	llmClient := llm.NewLLMClient(ep)
	model := ep.Model

	gitRunner := gitcmd.New(opts.maxGitProcs)

	collector := tool.NewCommentCollector()
	mode := tool.ParseReviewMode(opts.from, opts.to, opts.commit)
	ref, _ := mode.RefValue(opts.to, opts.commit)
	fileReader := &tool.FileReader{
		RepoDir: repoDir,
		Mode:    mode,
		Ref:     ref,
		Runner:  gitRunner,
	}
	tools := buildToolRegistry(collector, fileReader)

	ag := agent.New(agent.Args{
		RepoDir:               repoDir,
		From:                  opts.from,
		To:                    opts.to,
		Commit:                opts.commit,
		Template:              *tpl,
		SystemRule:            resolver,
		FileFilter:            fileFilter,
		LLMClient:             llmClient,
		Tools:                 tools,
		PlanToolDefs:          planToolDefs,
		MainToolDefs:          mainToolDefs,
		CommentCollector:      collector,
		CommentWorkerPool:     agent.NewCommentWorkerPool(opts.concurrency),
		MaxConcurrency:        opts.concurrency,
		ConcurrentTaskTimeout: opts.perFileTimeout,
		Model:                 model,
		Background:            opts.background,
		GitRunner:             gitRunner,
	})

	// Silence progress output during execution; restore before Summary in agent mode.
	var unsilence func()
	if opts.outputFormat == "json" || opts.audience == "agent" {
		unsilence = stdout.Quiet()
		defer func() {
			if unsilence != nil {
				unsilence()
			}
		}()
	}

	ctx, span := telemetry.StartSpan(context.Background(), "review.run")
	defer span.End()
	startTime := time.Now()

	comments, err := ag.Run(ctx)
	if err != nil {
		telemetry.SetAttr(span, "error", err.Error())
		return fmt.Errorf("review failed: %w", err)
	}

	// Resolve line numbers by matching existing_code against diff hunks.
	comments = diff.ResolveLineNumbers(comments, ag.Diffs())

	// Record summary metrics (files_reviewed is refined by agent.Run).
	duration := time.Since(startTime)
	telemetry.RecordReviewDuration(ctx, duration)
	if len(comments) > 0 {
		telemetry.RecordCommentsGenerated(ctx, int64(len(comments)))
	}

	// If no files were reviewed (e.g. workspace has no changes), inform the caller in JSON mode.
	if opts.outputFormat == "json" && len(comments) == 0 && ag.FilesReviewed() == 0 {
		return outputJSONNoFiles()
	}

	// In agent mode (text output), restore stdout so Summary reaches the terminal.
	if opts.audience == "agent" && opts.outputFormat != "json" && unsilence != nil {
		unsilence()
		unsilence = nil
	}

	if opts.outputFormat != "json" {
		telemetry.PrintTraceSummary(ag.FilesReviewed(), int64(len(comments)), ag.TotalInputTokens(), ag.TotalOutputTokens(), ag.TotalTokensUsed(), ag.TotalCacheReadTokens(), ag.TotalCacheWriteTokens(), duration)
	}

	if opts.outputFormat == "json" {
		if err := outputJSONWithWarnings(comments, ag.Warnings(), ag.FilesReviewed(), ag.TotalInputTokens(), ag.TotalOutputTokens(), ag.TotalTokensUsed(), ag.TotalCacheReadTokens(), ag.TotalCacheWriteTokens(), duration); err != nil {
			return err
		}
	} else {
		outputTextWithWarnings(comments, ag.Warnings())
	}

	if err := postToGitLabIfConfigured(comments, ag.Warnings(), opts, repoCfg, repoDir); err != nil {
		fmt.Fprintf(os.Stderr, "[ocr] GitLab posting failed: %v\n", err)
	}

	return nil
}

func resolveRepoDir(input string) (string, error) {
	if input == "" {
		var err error
		input, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get working directory: %w", err)
		}
	}
	absPath, err := filepath.Abs(input)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	out, err := runGitCmd(absPath, "rev-parse", "--git-dir")
	if err != nil || len(out) == 0 {
		return "", fmt.Errorf("%s is not a git repository", absPath)
	}
	return absPath, nil
}

// requireGitRepo validates that the given directory is part of a git repository.
func requireGitRepo(dir string) error {
	repoDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	out, err := runGitCmd(repoDir, "rev-parse", "--git-dir")
	if err != nil || len(out) == 0 {
		return fmt.Errorf("%s is not a git repository, code review requires a valid git repository", repoDir)
	}
	return nil
}

func validateReviewRefs(repoDir string, opts reviewOptions) error {
	refs := []struct {
		flag string
		ref  string
	}{
		{"--from", opts.from},
		{"--to", opts.to},
		{"--commit", opts.commit},
	}
	for _, item := range refs {
		if item.ref == "" {
			continue
		}
		if strings.HasPrefix(item.ref, "-") {
			return fmt.Errorf("%s value %q is not a valid git ref: refs must not start with '-'", item.flag, item.ref)
		}
		if out, err := runGitCmd(repoDir, "rev-parse", "--verify", "--end-of-options", item.ref+"^{commit}"); err != nil {
			msg := strings.TrimSpace(string(out))
			if msg != "" {
				return fmt.Errorf("%s value %q is not a valid commit ref: %s", item.flag, item.ref, msg)
			}
			return fmt.Errorf("%s value %q is not a valid commit ref", item.flag, item.ref)
		}
	}
	return nil
}

func runPreview(repoDir string, opts reviewOptions, fileFilter *rules.FileFilter) error {
	gitRunner := gitcmd.New(opts.maxGitProcs)
	ag := agent.New(agent.Args{
		RepoDir:    repoDir,
		From:       opts.from,
		To:         opts.to,
		Commit:     opts.commit,
		FileFilter: fileFilter,
		GitRunner:  gitRunner,
	})

	preview, err := ag.Preview(context.Background())
	if err != nil {
		return fmt.Errorf("preview failed: %w", err)
	}

	outputPreviewText(preview)
	return nil
}

// postToGitLabIfConfigured checks config/flags and posts to GitLab if appropriate.
func postToGitLabIfConfigured(comments []model.LlmComment, warnings []agent.AgentWarning, opts reviewOptions, repoCfg *RepoConfig, repoDir string) error {
	if opts.noPost {
		return nil
	}

	shouldPost := opts.post
	if !shouldPost && repoCfg != nil && repoCfg.Review != nil {
		shouldPost = repoCfg.Review.AutoPost
	}
	if !shouldPost {
		return nil
	}

	if repoCfg == nil || repoCfg.GitLab == nil || repoCfg.GitLab.Token == "" {
		return fmt.Errorf("no GitLab token configured. Run 'ocr setup' first")
	}
	if repoCfg.GitLab.ProjectID == "" {
		return fmt.Errorf("no GitLab project ID configured. Run 'ocr setup' first")
	}

	gitlabURL := repoCfg.GitLab.URL
	if gitlabURL == "" {
		gitlabURL = detectGitLabURL(repoDir)
	}
	client := newGitLabClient(repoCfg.GitLab.Token, repoCfg.GitLab.ProjectID, gitlabURL)

	if err := client.autoDetectMR(repoDir); err != nil {
		return fmt.Errorf("auto-detect MR: %w", err)
	}

	if len(comments) == 0 {
		fmt.Fprintf(os.Stderr, "[ocr] Posting review result to MR !%d...\n", client.MRIIID)
	} else {
		fmt.Fprintf(os.Stderr, "[ocr] Posting %d comment(s) to MR !%d...\n", len(comments), client.MRIIID)
	}
	return client.PostComments(comments, warnings)
}

func buildToolRegistry(collector *tool.CommentCollector, fr *tool.FileReader) *tool.Registry {
	reg := tool.NewRegistry()
	reg.Register(tool.NewFileRead(fr))
	reg.Register(tool.NewFileFind(fr))
	reg.Register(tool.NewFileReadDiff(tool.DiffMap{}))
	reg.Register(tool.NewCodeSearch(fr))
	reg.Register(&tool.CodeCommentProvider{Collector: collector})
	return reg
}
