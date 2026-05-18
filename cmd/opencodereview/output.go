package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/open-code-review/open-code-review/internal/agent"
	"github.com/open-code-review/open-code-review/internal/model"
	"github.com/open-code-review/open-code-review/internal/suggestdiff"
)

func outputText(comments []model.LlmComment) {
	if len(comments) == 0 {
		fmt.Println("No comments generated. Looks good to me.")
		return
	}
	for _, c := range comments {
		renderComment(c)
	}
}

func outputTextWithWarnings(comments []model.LlmComment, warnings []agent.AgentWarning) {
	outputText(comments)
	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "[ocr] WARNING [%s] %s: %s\n", w.Type, w.File, w.Message)
		}
	}
}

func renderComment(comment model.LlmComment) {
	lines := buildDiffLines(comment)
	if len(lines) == 0 && comment.Content == "" {
		return
	}

	fmt.Printf("\n\033[2m─── %s:%d-%d ───\033[0m\n", comment.Path, comment.StartLine, comment.EndLine)

	if comment.Content != "" {
		for _, ln := range wrapByRunes(comment.Content, 100) {
			fmt.Printf("%s\n", ln)
		}
		fmt.Println()
	}

	if len(lines) > 0 {
		for _, dl := range lines {
			switch dl.Type {
			case suggestdiff.DiffAdded:
				printDiffLine("+", dl.Content, "\033[92m", "\033[48;2;0;60;0m")
			case suggestdiff.DiffDeleted:
				printDiffLine("-", dl.Content, "\033[91m", "\033[48;2;70;0;0m")
			case suggestdiff.DiffContext:
				printDiffLine(" ", dl.Content, "\033[2m", "\033[48;2;38;38;38m")
			}
		}
	}

	fmt.Println()
}

// printDiffLine renders a single diff line with colored prefix and background on content.
func printDiffLine(prefix, content, fgColor, bgColor string) {
	fmt.Printf("%s%s%s %s%s\033[0m\n", fgColor+bgColor, prefix, "\033[0m"+bgColor, content, "\033[0m")
}

// wrapByRunes splits text into lines that fit within maxWidth **rune** columns.
// Respects existing newlines and wraps at word boundaries.
func wrapByRunes(text string, maxW int) []string {
	if text == "" {
		return nil
	}
	var result []string
	for _, para := range strings.Split(text, "\n") {
		result = append(result, wrapSingleRuneLine(para, maxW)...)
	}
	return result
}

// wrapSingleRuneLine breaks one paragraph (no newlines) into rune-width-constrained lines.
func wrapSingleRuneLine(line string, maxW int) []string {
	runes := []rune(line)
	if visibleRunesLen(runes) <= maxW {
		return []string{line}
	}
	var result []string
	for len(runes) > 0 {
		cut := runeWrapCut(runes, maxW)
		result = append(result, string(runes[:cut]))
		runes = runes[cut:]
		// trim leading spaces of next segment
		for len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	return result
}

// runeWrapCut returns a rune index suitable for breaking the line at ~maxW display width.
func runeWrapCut(runes []rune, maxW int) int {
	if visibleRunesLen(runes) <= maxW {
		return len(runes)
	}
	best := maxW
	if best >= len(runes) {
		return len(runes)
	}
	for i := best; i > 0; i-- {
		if runes[i] == ' ' || runes[i] == '\t' {
			return i
		}
	}
	return best
}

func visibleRunesLen(runes []rune) int {
	n := 0
	for _, r := range runes {
		if r >= 32 && r != 127 {
			n++
		}
	}
	return n
}

func splitToLines(s string) []string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func buildDiffLines(comment model.LlmComment) []suggestdiff.DiffLine {
	if comment.SuggestionCode == "" || comment.ExistingCode == "" {
		return nil
	}
	oldLines := splitToLines(comment.ExistingCode)
	newLines := splitToLines(comment.SuggestionCode)
	return suggestdiff.ComputeLineDiff(oldLines, newLines)
}

type jsonOutput struct {
	Status   string              `json:"status"`
	Message  string              `json:"message,omitempty"`
	Comments []model.LlmComment  `json:"comments"`
	Warnings []agent.AgentWarning `json:"warnings,omitempty"`
}

func outputJSON(comments []model.LlmComment) error {
	out := jsonOutput{
		Status:   "success",
		Comments: comments,
	}
	if len(comments) == 0 {
		out.Message = "No comments generated. Looks good to me."
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func outputJSONWithWarnings(comments []model.LlmComment, warnings []agent.AgentWarning) error {
	out := jsonOutput{
		Status:   "success",
		Comments: comments,
	}
	if len(comments) == 0 {
		out.Message = "No comments generated. Looks good to me."
	}
	if len(warnings) > 0 {
		out.Warnings = warnings
		out.Status = "completed_with_warnings"
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func outputJSONNoFiles() error {
	out := jsonOutput{
		Status:   "skipped",
		Message:  "No supported files changed.",
		Comments: []model.LlmComment{},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
