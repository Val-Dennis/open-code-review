package main

import (
	"strings"
	"testing"
)

func TestValidateReviewRefsRejectsOptionLikeCommit(t *testing.T) {
	err := validateReviewRefs(t.TempDir(), reviewOptions{commit: "-O./pwn.sh"})
	if err == nil {
		t.Fatal("expected option-like --commit ref to be rejected")
	}
	if !strings.Contains(err.Error(), "--commit") || !strings.Contains(err.Error(), "must not start with '-'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateReviewRefsRejectsOptionLikeRangeRef(t *testing.T) {
	err := validateReviewRefs(t.TempDir(), reviewOptions{to: "-O./pwn.sh"})
	if err == nil {
		t.Fatal("expected option-like --to ref to be rejected")
	}
	if !strings.Contains(err.Error(), "--to") || !strings.Contains(err.Error(), "must not start with '-'") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseReviewFlagsRejectsToWithoutFrom(t *testing.T) {
	_, err := parseReviewFlags([]string{"--to", "HEAD"})
	if err == nil {
		t.Fatal("expected --to without --from to fail")
	}
	if !strings.Contains(err.Error(), "--from is required when --to is specified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseReviewFlagsRejectsFromWithoutTo(t *testing.T) {
	_, err := parseReviewFlags([]string{"--from", "main"})
	if err == nil {
		t.Fatal("expected --from without --to to fail")
	}
	if !strings.Contains(err.Error(), "--to is required when --from is specified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseReviewFlagsAllowsFromAndTo(t *testing.T) {
	opts, err := parseReviewFlags([]string{"--from", "main", "--to", "HEAD"})
	if err != nil {
		t.Fatalf("expected --from/--to to pass, got: %v", err)
	}
	if opts.from != "main" || opts.to != "HEAD" {
		t.Fatalf("unexpected opts: from=%q to=%q", opts.from, opts.to)
	}
}

// applyToDefault mirrors the defaulting logic used in runReview/runFastReview
// after config merge: when --from is set but --to is empty, default to HEAD.
func applyToDefault(opts *reviewOptions) {
	if opts.from != "" && opts.to == "" {
		opts.to = "HEAD"
	}
}

func TestApplyToDefault_FromWithoutTo(t *testing.T) {
	opts := reviewOptions{from: "origin/main"}
	applyToDefault(&opts)
	if opts.to != "HEAD" {
		t.Errorf("to = %q, want %q", opts.to, "HEAD")
	}
}

func TestApplyToDefault_BothSet(t *testing.T) {
	opts := reviewOptions{from: "origin/main", to: "feature"}
	applyToDefault(&opts)
	if opts.to != "feature" {
		t.Errorf("to = %q, want %q (should not override explicit value)", opts.to, "feature")
	}
}

func TestApplyToDefault_NeitherSet(t *testing.T) {
	opts := reviewOptions{}
	applyToDefault(&opts)
	if opts.to != "" {
		t.Errorf("to = %q, want empty (workspace mode)", opts.to)
	}
}

func TestApplyToDefault_OnlyToSet(t *testing.T) {
	opts := reviewOptions{to: "HEAD"}
	applyToDefault(&opts)
	if opts.to != "HEAD" {
		t.Errorf("to = %q, want %q", opts.to, "HEAD")
	}
}

func TestRepoConfigFromDefaultsToHEAD(t *testing.T) {
	opts := reviewOptions{}
	rc := &ReviewSection{From: "origin/main"}
	if opts.from == "" && rc.From != "" {
		opts.from = rc.From
	}
	if opts.to == "" && rc.To != "" {
		opts.to = rc.To
	}
	applyToDefault(&opts)

	if opts.from != "origin/main" {
		t.Errorf("from = %q, want %q", opts.from, "origin/main")
	}
	if opts.to != "HEAD" {
		t.Errorf("to = %q, want %q", opts.to, "HEAD")
	}
}

func TestRepoConfigFromAndTo(t *testing.T) {
	opts := reviewOptions{}
	rc := &ReviewSection{From: "origin/main", To: "HEAD"}
	if opts.from == "" && rc.From != "" {
		opts.from = rc.From
	}
	if opts.to == "" && rc.To != "" {
		opts.to = rc.To
	}
	applyToDefault(&opts)

	if opts.from != "origin/main" {
		t.Errorf("from = %q, want %q", opts.from, "origin/main")
	}
	if opts.to != "HEAD" {
		t.Errorf("to = %q, want %q", opts.to, "HEAD")
	}
}

func TestRepoConfigCustomTo(t *testing.T) {
	opts := reviewOptions{}
	rc := &ReviewSection{From: "origin/develop", To: "staging"}
	if opts.from == "" && rc.From != "" {
		opts.from = rc.From
	}
	if opts.to == "" && rc.To != "" {
		opts.to = rc.To
	}
	applyToDefault(&opts)

	if opts.from != "origin/develop" {
		t.Errorf("from = %q, want %q", opts.from, "origin/develop")
	}
	if opts.to != "staging" {
		t.Errorf("to = %q, want %q (explicit config To should be used)", opts.to, "staging")
	}
}

func TestCLIFromToOverridesDefault(t *testing.T) {
	opts, err := parseReviewFlags([]string{"--from", "origin/develop", "--to", "my-branch"})
	if err != nil {
		t.Fatalf("parseReviewFlags: %v", err)
	}
	applyToDefault(&opts)

	if opts.from != "origin/develop" {
		t.Errorf("from = %q, want %q", opts.from, "origin/develop")
	}
	if opts.to != "my-branch" {
		t.Errorf("to = %q, want %q (explicit --to should not be overridden)", opts.to, "my-branch")
	}
}
