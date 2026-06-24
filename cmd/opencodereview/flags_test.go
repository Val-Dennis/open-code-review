package main

import "testing"

func TestParseReviewFlagsModelOverride(t *testing.T) {
	opts, err := parseReviewFlags([]string{"--model", "claude-opus-4-6"})
	if err != nil {
		t.Fatalf("parseReviewFlags: %v", err)
	}

	if opts.model != "claude-opus-4-6" {
		t.Errorf("model = %q, want %q", opts.model, "claude-opus-4-6")
	}
	if opts.outputFormat != "text" {
		t.Errorf("outputFormat = %q, want %q", opts.outputFormat, "text")
	}
	if opts.audience != "human" {
		t.Errorf("audience = %q, want %q", opts.audience, "human")
	}
}

func TestParseReviewFlagsPostNoPostConflict(t *testing.T) {
	_, err := parseReviewFlags([]string{"--post", "--no-post"})
	if err == nil {
		t.Fatal("expected error for --post and --no-post together")
	}
}

func TestParseReviewFlagsPostAndNoPost(t *testing.T) {
	opts, err := parseReviewFlags([]string{"--post"})
	if err != nil {
		t.Fatalf("parseReviewFlags: %v", err)
	}
	if !opts.post || opts.noPost {
		t.Fatalf("post = %v, noPost = %v", opts.post, opts.noPost)
	}

	opts, err = parseReviewFlags([]string{"--no-post"})
	if err != nil {
		t.Fatalf("parseReviewFlags: %v", err)
	}
	if opts.post || !opts.noPost {
		t.Fatalf("post = %v, noPost = %v", opts.post, opts.noPost)
	}
}
