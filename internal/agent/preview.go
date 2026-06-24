package agent

import (
	"context"
	"fmt"
	"strings"

	allowedext "github.com/open-code-review/open-code-review/internal/config/allowlist"
	"github.com/open-code-review/open-code-review/internal/config/rules"
	"github.com/open-code-review/open-code-review/internal/model"
)

// ExcludeReason describes why a file was excluded from review.
type ExcludeReason string

const (
	ExcludeNone        ExcludeReason = ""
	ExcludeUserRule    ExcludeReason = "user_exclude"
	ExcludeExtension   ExcludeReason = "unsupported_ext"
	ExcludeDefaultPath ExcludeReason = "default_path"
	ExcludeDeleted     ExcludeReason = "deleted"
	ExcludeBinary      ExcludeReason = "binary"
)

// DiffPreviewEntry is one file's preview record.
type DiffPreviewEntry struct {
	Path          string        `json:"path"`
	Status        string        `json:"status"`
	Insertions    int64         `json:"insertions"`
	Deletions     int64         `json:"deletions"`
	WillReview    bool          `json:"will_review"`
	ExcludeReason ExcludeReason `json:"exclude_reason,omitempty"`
}

// DiffPreview is the full preview result.
type DiffPreview struct {
	Entries         []DiffPreviewEntry `json:"files"`
	TotalInsertions int64              `json:"total_insertions"`
	TotalDeletions  int64              `json:"total_deletions"`
	TotalFiles      int                `json:"total_files"`
	ReviewableCount int                `json:"reviewable_count"`
	ExcludedCount   int                `json:"excluded_count"`
}

// WhyExcluded determines whether a diff should be excluded from review
// and returns the specific reason. It applies the same filter algorithm
// used by the agent path, so callers (including fast-mode) share one
// implementation.
func WhyExcluded(filter *rules.FileFilter, d model.Diff) ExcludeReason {
	if d.IsBinary {
		return ExcludeBinary
	}

	path := EffectivePath(d)

	if filter != nil && filter.IsUserExcluded(path) {
		return ExcludeUserRule
	}

	if filter != nil && filter.HasInclude() && filter.IsUserIncluded(path) {
		return ExcludeNone
	}

	ext := extFromPath(path)
	if ext != "" && !allowedext.IsAllowedExt(ext) {
		return ExcludeExtension
	}

	if allowedext.IsExcludedPath(path) {
		return ExcludeDefaultPath
	}

	return ExcludeNone
}

// FilterDiffs returns only the diffs that should be reviewed, applying
// user include/exclude patterns and default extension/path filters.
func FilterDiffs(filter *rules.FileFilter, diffs []model.Diff) []model.Diff {
	var kept []model.Diff
	for _, d := range diffs {
		if WhyExcluded(filter, d) == ExcludeNone {
			kept = append(kept, d)
		}
	}
	return kept
}

func (a *Agent) whyExcluded(d model.Diff) ExcludeReason {
	return WhyExcluded(a.args.FileFilter, d)
}

// Preview loads diffs and applies the filter algorithm, returning structured
// preview data without dispatching any LLM calls.
func (a *Agent) Preview(ctx context.Context) (*DiffPreview, error) {
	if err := a.loadDiffs(ctx); err != nil {
		return nil, fmt.Errorf("load diffs: %w", err)
	}

	result := &DiffPreview{
		TotalInsertions: a.totalInsertions,
		TotalDeletions:  a.totalDeletions,
		TotalFiles:      len(a.diffs),
	}

	for _, d := range a.diffs {
		path := EffectivePath(d)
		entry := DiffPreviewEntry{
			Path:       path,
			Insertions: d.Insertions,
			Deletions:  d.Deletions,
			Status:     diffStatus(d),
		}

		reason := a.whyExcluded(d)
		if reason == ExcludeNone && d.IsDeleted {
			reason = ExcludeDeleted
		}

		entry.WillReview = reason == ExcludeNone
		entry.ExcludeReason = reason

		if entry.WillReview {
			result.ReviewableCount++
		} else {
			result.ExcludedCount++
		}

		result.Entries = append(result.Entries, entry)
	}

	return result, nil
}

// EffectivePath returns the relevant path for a diff entry,
// falling back to OldPath for deleted files.
func EffectivePath(d model.Diff) string {
	if d.NewPath == "/dev/null" {
		return d.OldPath
	}
	return d.NewPath
}

// extFromPath returns the file extension with leading dot, lowercased.
func extFromPath(path string) string {
	basename := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		basename = path[idx+1:]
	}
	dot := strings.LastIndex(basename, ".")
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(basename[dot:])
}

func diffStatus(d model.Diff) string {
	switch {
	case d.IsBinary:
		return "binary"
	case d.IsNew:
		return "added"
	case d.IsDeleted:
		return "deleted"
	case d.IsRenamed:
		return "renamed"
	case d.OldPath != d.NewPath && d.OldPath != "" && d.OldPath != "/dev/null":
		return "renamed"
	default:
		return "modified"
	}
}
