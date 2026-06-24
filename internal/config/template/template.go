// Package template loads and validates task prompt templates for the code review agent.
package template

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Template holds the native agent task template configuration.
type Template struct {
	MainTask              LlmConversation  `json:"MAIN_TASK"`
	PlanTask              *LlmConversation `json:"PLAN_TASK,omitempty"`
	MemoryCompressionTask LlmConversation  `json:"MEMORY_COMPRESSION_TASK"`
	MaxTokens             int              `json:"MAX_TOKENS"`
	MaxToolRequestTimes   int              `json:"MAX_TOOL_REQUEST_TIMES"`
	PlanModeLineThreshold int              `json:"PLAN_MODE_LINE_THRESHOLD"`
	ReLocationTask        *LlmConversation `json:"RE_LOCATION_TASK,omitempty"`
	ReviewFilterTask      *LlmConversation `json:"REVIEW_FILTER_TASK,omitempty"`
	FastTask              *LlmConversation `json:"FAST_TASK,omitempty"`
}

//go:embed task_template.json prompts/*
var templateFS embed.FS

type manifestMessage struct {
	Role       string `json:"role"`
	PromptFile string `json:"prompt_file"`
}

type manifestConversation struct {
	Timeout  int               `json:"timeout"`
	Messages []manifestMessage `json:"messages"`
}

type templateManifest struct {
	MainTask              manifestConversation  `json:"MAIN_TASK"`
	PlanTask              *manifestConversation `json:"PLAN_TASK,omitempty"`
	MemoryCompressionTask manifestConversation  `json:"MEMORY_COMPRESSION_TASK"`
	MaxTokens             int                   `json:"MAX_TOKENS"`
	MaxToolRequestTimes   int                   `json:"MAX_TOOL_REQUEST_TIMES"`
	PlanModeLineThreshold int                   `json:"PLAN_MODE_LINE_THRESHOLD"`
	ReLocationTask        *manifestConversation `json:"RE_LOCATION_TASK,omitempty"`
	ReviewFilterTask      *manifestConversation `json:"REVIEW_FILTER_TASK,omitempty"`
	FastTask              *manifestConversation `json:"FAST_TASK,omitempty"`
}

func resolveConversation(m manifestConversation) (LlmConversation, error) {
	conv := LlmConversation{Timeout: m.Timeout}
	conv.Messages = make([]ChatMessage, len(m.Messages))
	for i, mm := range m.Messages {
		data, err := templateFS.ReadFile("prompts/" + mm.PromptFile)
		if err != nil {
			return LlmConversation{}, fmt.Errorf("read prompt file %q: %w", mm.PromptFile, err)
		}
		conv.Messages[i] = ChatMessage{
			Role:    mm.Role,
			Content: strings.TrimRight(string(data), "\r\n"),
		}
	}
	return conv, nil
}

func resolveOptionalConversation(m *manifestConversation, name string) (*LlmConversation, error) {
	if m == nil {
		return nil, nil
	}
	conv, err := resolveConversation(*m)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &conv, nil
}

// LoadDefault parses the embedded task_template.json and resolves prompt file references.
func LoadDefault() (*Template, error) {
	data, err := templateFS.ReadFile("task_template.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded task_template.json: %w", err)
	}
	var m templateManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal task_template manifest: %w", err)
	}

	var tpl Template
	tpl.MaxTokens = m.MaxTokens
	tpl.MaxToolRequestTimes = m.MaxToolRequestTimes
	tpl.PlanModeLineThreshold = m.PlanModeLineThreshold

	if tpl.MainTask, err = resolveConversation(m.MainTask); err != nil {
		return nil, fmt.Errorf("MAIN_TASK: %w", err)
	}
	if tpl.PlanTask, err = resolveOptionalConversation(m.PlanTask, "PLAN_TASK"); err != nil {
		return nil, err
	}
	if tpl.MemoryCompressionTask, err = resolveConversation(m.MemoryCompressionTask); err != nil {
		return nil, fmt.Errorf("MEMORY_COMPRESSION_TASK: %w", err)
	}
	if tpl.ReLocationTask, err = resolveOptionalConversation(m.ReLocationTask, "RE_LOCATION_TASK"); err != nil {
		return nil, err
	}
	if tpl.ReviewFilterTask, err = resolveOptionalConversation(m.ReviewFilterTask, "REVIEW_FILTER_TASK"); err != nil {
		return nil, err
	}
	if tpl.FastTask, err = resolveOptionalConversation(m.FastTask, "FAST_TASK"); err != nil {
		return nil, err
	}
	return &tpl, nil
}

// applyLanguage appends instruction to all system-role messages in conv.
func applyLanguage(conv *LlmConversation, instruction string) {
	for i := range conv.Messages {
		if conv.Messages[i].Role == "system" {
			conv.Messages[i].Content += instruction
		}
	}
}

// resolveLang returns the resolved language name for the instruction.
func resolveLang(lang string) string {
	if lang == "" {
		return "English"
	}
	return lang
}

// ApplyLanguage injects a language directive into all system-role messages
// across MAIN_TASK, PLAN_TASK (if set), and MEMORY_COMPRESSION_TASK.
func (t *Template) ApplyLanguage(lang string) {
	instruction := "\n\nAlways respond in " + resolveLang(lang) + "."
	applyLanguage(&t.MainTask, instruction)
	if t.PlanTask != nil {
		applyLanguage(t.PlanTask, instruction)
	}
	applyLanguage(&t.MemoryCompressionTask, instruction)
}
func (t *Template) Validate() error {
	if t.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive")
	}
	if t.MaxToolRequestTimes <= 0 {
		return fmt.Errorf("max_tool_request_times must be positive")
	}
	if len(t.MainTask.Messages) == 0 {
		return fmt.Errorf("main_task.messages must not be empty")
	}
	return nil
}

// PromptsConfig holds optional user-provided prompt overrides.
// Each field corresponds to a task's system or user prompt. Values can be
// inline text or a "file:<path>" reference that is resolved at load time.
type PromptsConfig struct {
	MainTaskSystem              string `json:"main_task_system,omitempty"`
	MainTaskUser                string `json:"main_task_user,omitempty"`
	PlanTaskSystem              string `json:"plan_task_system,omitempty"`
	PlanTaskUser                string `json:"plan_task_user,omitempty"`
	MemoryCompressionTaskSystem string `json:"memory_compression_task_system,omitempty"`
	MemoryCompressionTaskUser   string `json:"memory_compression_task_user,omitempty"`
	ReviewFilterTaskSystem      string `json:"review_filter_task_system,omitempty"`
	ReviewFilterTaskUser        string `json:"review_filter_task_user,omitempty"`
	ReLocationTaskSystem        string `json:"re_location_task_system,omitempty"`
	ReLocationTaskUser          string `json:"re_location_task_user,omitempty"`
	FastModeSystem              string `json:"fast_mode_system,omitempty"`
}

// promptOverride pairs a raw config value with a pointer to the target message.
type promptOverride struct {
	raw  string
	msg  *ChatMessage
	name string
}

// ResolvePromptValue resolves a single prompt value. If the value starts with
// "file:", the remainder is treated as a file path (resolved relative to baseDir).
// Otherwise the value is returned as-is.
func ResolvePromptValue(raw, baseDir string) (string, error) {
	if !strings.HasPrefix(raw, "file:") {
		return raw, nil
	}
	path := strings.TrimPrefix(raw, "file:")
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read prompt file %q: %w", path, err)
	}
	return strings.TrimRight(string(data), "\r\n"), nil
}

// ApplyPromptOverrides replaces template messages with user-supplied overrides.
// baseDir is used to resolve relative "file:" paths.
func (t *Template) ApplyPromptOverrides(cfg *PromptsConfig, baseDir string) error {
	if cfg == nil {
		return nil
	}

	overrides := t.collectOverrides(cfg)
	for _, o := range overrides {
		if o.raw == "" {
			continue
		}
		resolved, err := ResolvePromptValue(o.raw, baseDir)
		if err != nil {
			return fmt.Errorf("prompt %s: %w", o.name, err)
		}
		o.msg.Content = resolved
	}
	return nil
}

// FastTaskSystemPrompt returns the default fast-mode system prompt loaded from
// the embedded template. Returns empty string if not configured.
func (t *Template) FastTaskSystemPrompt() string {
	if t.FastTask == nil || len(t.FastTask.Messages) == 0 {
		return ""
	}
	return t.FastTask.Messages[0].Content
}

// collectOverrides maps each PromptsConfig field to the corresponding template message.
// Messages are referenced by index: [0] = system, [1] = user for every task.
func (t *Template) collectOverrides(cfg *PromptsConfig) []promptOverride {
	var out []promptOverride

	addPair := func(conv *LlmConversation, sys, usr, prefix string) {
		if conv == nil || len(conv.Messages) < 2 {
			return
		}
		if sys != "" {
			out = append(out, promptOverride{raw: sys, msg: &conv.Messages[0], name: prefix + "_system"})
		}
		if usr != "" {
			out = append(out, promptOverride{raw: usr, msg: &conv.Messages[1], name: prefix + "_user"})
		}
	}

	addPair(&t.MainTask, cfg.MainTaskSystem, cfg.MainTaskUser, "main_task")
	addPair(t.PlanTask, cfg.PlanTaskSystem, cfg.PlanTaskUser, "plan_task")
	addPair(&t.MemoryCompressionTask, cfg.MemoryCompressionTaskSystem, cfg.MemoryCompressionTaskUser, "memory_compression_task")
	addPair(t.ReviewFilterTask, cfg.ReviewFilterTaskSystem, cfg.ReviewFilterTaskUser, "review_filter_task")
	addPair(t.ReLocationTask, cfg.ReLocationTaskSystem, cfg.ReLocationTaskUser, "re_location_task")

	if cfg.FastModeSystem != "" && t.FastTask != nil && len(t.FastTask.Messages) >= 1 {
		out = append(out, promptOverride{raw: cfg.FastModeSystem, msg: &t.FastTask.Messages[0], name: "fast_mode_system"})
	}

	return out
}

// MergePrompts returns a new PromptsConfig where non-empty fields from override
// take precedence over base. Either argument may be nil.
func MergePrompts(base, override *PromptsConfig) *PromptsConfig {
	if base == nil && override == nil {
		return nil
	}
	merged := &PromptsConfig{}
	if base != nil {
		*merged = *base
	}
	if override == nil {
		return merged
	}
	if override.MainTaskSystem != "" {
		merged.MainTaskSystem = override.MainTaskSystem
	}
	if override.MainTaskUser != "" {
		merged.MainTaskUser = override.MainTaskUser
	}
	if override.PlanTaskSystem != "" {
		merged.PlanTaskSystem = override.PlanTaskSystem
	}
	if override.PlanTaskUser != "" {
		merged.PlanTaskUser = override.PlanTaskUser
	}
	if override.MemoryCompressionTaskSystem != "" {
		merged.MemoryCompressionTaskSystem = override.MemoryCompressionTaskSystem
	}
	if override.MemoryCompressionTaskUser != "" {
		merged.MemoryCompressionTaskUser = override.MemoryCompressionTaskUser
	}
	if override.ReviewFilterTaskSystem != "" {
		merged.ReviewFilterTaskSystem = override.ReviewFilterTaskSystem
	}
	if override.ReviewFilterTaskUser != "" {
		merged.ReviewFilterTaskUser = override.ReviewFilterTaskUser
	}
	if override.ReLocationTaskSystem != "" {
		merged.ReLocationTaskSystem = override.ReLocationTaskSystem
	}
	if override.ReLocationTaskUser != "" {
		merged.ReLocationTaskUser = override.ReLocationTaskUser
	}
	if override.FastModeSystem != "" {
		merged.FastModeSystem = override.FastModeSystem
	}
	return merged
}

// LlmConversation mirrors LlmConversation from the Java side — a preset prompt with settings.
type LlmConversation struct {
	Timeout  int           `json:"timeout"`
	Messages []ChatMessage `json:"messages"`
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
