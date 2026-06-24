package template

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDefault_FieldsPopulated(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	if len(tpl.MainTask.Messages) != 2 {
		t.Errorf("MainTask.Messages length = %d, want 2", len(tpl.MainTask.Messages))
	}
	for i, msg := range tpl.MainTask.Messages {
		if msg.Content == "" {
			t.Errorf("MainTask.Messages[%d].Content is empty", i)
		}
	}
	if tpl.MainTask.Timeout != 120 {
		t.Errorf("MainTask.Timeout = %d, want 120", tpl.MainTask.Timeout)
	}
	if tpl.PlanTask == nil {
		t.Fatal("PlanTask is nil, expected non-nil")
	}
	if len(tpl.PlanTask.Messages) != 2 {
		t.Errorf("PlanTask.Messages length = %d, want 2", len(tpl.PlanTask.Messages))
	}
	if tpl.ReLocationTask == nil {
		t.Fatal("ReLocationTask is nil, expected non-nil")
	}
	if tpl.ReviewFilterTask == nil {
		t.Fatal("ReviewFilterTask is nil, expected non-nil")
	}
	if tpl.FastTask == nil {
		t.Fatal("FastTask is nil, expected non-nil")
	}
	if len(tpl.FastTask.Messages) != 1 {
		t.Errorf("FastTask.Messages length = %d, want 1", len(tpl.FastTask.Messages))
	}
	if tpl.FastTask.Messages[0].Role != "system" {
		t.Errorf("FastTask.Messages[0].Role = %q, want %q", tpl.FastTask.Messages[0].Role, "system")
	}
	if tpl.MaxTokens != 58888 {
		t.Errorf("MaxTokens = %d, want 58888", tpl.MaxTokens)
	}
	if tpl.MaxToolRequestTimes != 30 {
		t.Errorf("MaxToolRequestTimes = %d, want 30", tpl.MaxToolRequestTimes)
	}
	if tpl.PlanModeLineThreshold != 50 {
		t.Errorf("PlanModeLineThreshold = %d, want 50", tpl.PlanModeLineThreshold)
	}
}

func TestLoadDefault_PlaceholdersPresent(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	tests := []struct {
		name        string
		content     string
		placeholder string
	}{
		{"MainTask user has current_file_path", tpl.MainTask.Messages[1].Content, "{{current_file_path}}"},
		{"MainTask user has diff", tpl.MainTask.Messages[1].Content, "{{diff}}"},
		{"PlanTask system has plan_tools", tpl.PlanTask.Messages[0].Content, "{{plan_tools}}"},
		{"MemoryCompression user has context", tpl.MemoryCompressionTask.Messages[1].Content, "{{context}}"},
		{"ReviewFilter user has comments", tpl.ReviewFilterTask.Messages[1].Content, "{{comments}}"},
		{"ReLocation user has diff (single brace)", tpl.ReLocationTask.Messages[1].Content, "{diff}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(tt.content, tt.placeholder) {
				t.Errorf("content does not contain %q", tt.placeholder)
			}
		})
	}
}

func TestValidate_PassesOnDefault(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	if err := tpl.Validate(); err != nil {
		t.Errorf("Validate() error: %v", err)
	}
}

func TestApplyLanguage(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	tpl.ApplyLanguage("Chinese")
	suffix := "\n\nAlways respond in Chinese."
	if !strings.HasSuffix(tpl.MainTask.Messages[0].Content, suffix) {
		t.Errorf("MainTask system message does not end with %q", suffix)
	}
	if !strings.HasSuffix(tpl.PlanTask.Messages[0].Content, suffix) {
		t.Errorf("PlanTask system message does not end with %q", suffix)
	}
	if !strings.HasSuffix(tpl.MemoryCompressionTask.Messages[0].Content, suffix) {
		t.Errorf("MemoryCompressionTask system message does not end with %q", suffix)
	}
}

func TestApplyLanguage_DefaultEnglish(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	tpl.ApplyLanguage("")
	suffix := "\n\nAlways respond in English."
	if !strings.HasSuffix(tpl.MainTask.Messages[0].Content, suffix) {
		t.Errorf("MainTask system message does not end with %q", suffix)
	}
}

func TestResolvePromptValue_Inline(t *testing.T) {
	val, err := ResolvePromptValue("hello world", "/tmp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "hello world" {
		t.Errorf("got %q, want %q", val, "hello world")
	}
}

func TestResolvePromptValue_FileReference(t *testing.T) {
	dir := t.TempDir()
	content := "custom system prompt\nwith multiple lines\n"
	if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	val, err := ResolvePromptValue("file:prompt.md", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "custom system prompt\nwith multiple lines"
	if val != want {
		t.Errorf("got %q, want %q", val, want)
	}
}

func TestResolvePromptValue_FileAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	absPath := filepath.Join(dir, "abs.md")
	if err := os.WriteFile(absPath, []byte("absolute content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	val, err := ResolvePromptValue("file:"+absPath, "/nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "absolute content" {
		t.Errorf("got %q, want %q", val, "absolute content")
	}
}

func TestResolvePromptValue_FileMissing(t *testing.T) {
	_, err := ResolvePromptValue("file:nonexistent.md", t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestApplyPromptOverrides_NilConfig(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	orig := tpl.MainTask.Messages[0].Content

	if err := tpl.ApplyPromptOverrides(nil, "/tmp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.MainTask.Messages[0].Content != orig {
		t.Error("nil config should not change any messages")
	}
}

func TestApplyPromptOverrides_InlineOverride(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	cfg := &PromptsConfig{
		MainTaskSystem: "custom system",
		MainTaskUser:   "custom user",
	}
	if err := tpl.ApplyPromptOverrides(cfg, "/tmp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.MainTask.Messages[0].Content != "custom system" {
		t.Errorf("system = %q, want %q", tpl.MainTask.Messages[0].Content, "custom system")
	}
	if tpl.MainTask.Messages[1].Content != "custom user" {
		t.Errorf("user = %q, want %q", tpl.MainTask.Messages[1].Content, "custom user")
	}
}

func TestApplyPromptOverrides_FileOverride(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sys.md"), []byte("from file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &PromptsConfig{
		PlanTaskSystem: "file:sys.md",
	}
	if err := tpl.ApplyPromptOverrides(cfg, dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.PlanTask.Messages[0].Content != "from file" {
		t.Errorf("plan system = %q, want %q", tpl.PlanTask.Messages[0].Content, "from file")
	}
}

func TestApplyPromptOverrides_PartialOverride(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}
	origUser := tpl.MainTask.Messages[1].Content

	cfg := &PromptsConfig{
		MainTaskSystem: "only system changed",
	}
	if err := tpl.ApplyPromptOverrides(cfg, "/tmp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.MainTask.Messages[0].Content != "only system changed" {
		t.Errorf("system = %q, want %q", tpl.MainTask.Messages[0].Content, "only system changed")
	}
	if tpl.MainTask.Messages[1].Content != origUser {
		t.Error("user message should be unchanged when not overridden")
	}
}

func TestApplyPromptOverrides_AllTasks(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	cfg := &PromptsConfig{
		MainTaskSystem:              "ms",
		MainTaskUser:                "mu",
		PlanTaskSystem:              "ps",
		PlanTaskUser:                "pu",
		MemoryCompressionTaskSystem: "mcs",
		MemoryCompressionTaskUser:   "mcu",
		ReviewFilterTaskSystem:      "rfs",
		ReviewFilterTaskUser:        "rfu",
		ReLocationTaskSystem:        "rls",
		ReLocationTaskUser:          "rlu",
	}
	if err := tpl.ApplyPromptOverrides(cfg, "/tmp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"MainTask system", tpl.MainTask.Messages[0].Content, "ms"},
		{"MainTask user", tpl.MainTask.Messages[1].Content, "mu"},
		{"PlanTask system", tpl.PlanTask.Messages[0].Content, "ps"},
		{"PlanTask user", tpl.PlanTask.Messages[1].Content, "pu"},
		{"MemoryCompression system", tpl.MemoryCompressionTask.Messages[0].Content, "mcs"},
		{"MemoryCompression user", tpl.MemoryCompressionTask.Messages[1].Content, "mcu"},
		{"ReviewFilter system", tpl.ReviewFilterTask.Messages[0].Content, "rfs"},
		{"ReviewFilter user", tpl.ReviewFilterTask.Messages[1].Content, "rfu"},
		{"ReLocation system", tpl.ReLocationTask.Messages[0].Content, "rls"},
		{"ReLocation user", tpl.ReLocationTask.Messages[1].Content, "rlu"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestMergePrompts_BothNil(t *testing.T) {
	if got := MergePrompts(nil, nil); got != nil {
		t.Errorf("MergePrompts(nil, nil) = %v, want nil", got)
	}
}

func TestMergePrompts_BaseOnly(t *testing.T) {
	base := &PromptsConfig{MainTaskSystem: "base sys"}
	got := MergePrompts(base, nil)
	if got.MainTaskSystem != "base sys" {
		t.Errorf("MainTaskSystem = %q, want %q", got.MainTaskSystem, "base sys")
	}
}

func TestMergePrompts_OverrideOnly(t *testing.T) {
	override := &PromptsConfig{PlanTaskUser: "override plan"}
	got := MergePrompts(nil, override)
	if got.PlanTaskUser != "override plan" {
		t.Errorf("PlanTaskUser = %q, want %q", got.PlanTaskUser, "override plan")
	}
}

func TestMergePrompts_OverrideWins(t *testing.T) {
	base := &PromptsConfig{
		MainTaskSystem: "base",
		MainTaskUser:   "base user",
		FastModeSystem: "base fast",
	}
	override := &PromptsConfig{
		MainTaskSystem: "override",
		FastModeSystem: "override fast",
	}
	got := MergePrompts(base, override)

	if got.MainTaskSystem != "override" {
		t.Errorf("MainTaskSystem = %q, want %q", got.MainTaskSystem, "override")
	}
	if got.MainTaskUser != "base user" {
		t.Errorf("MainTaskUser = %q, want %q (should keep base)", got.MainTaskUser, "base user")
	}
	if got.FastModeSystem != "override fast" {
		t.Errorf("FastModeSystem = %q, want %q", got.FastModeSystem, "override fast")
	}
}

func TestMergePrompts_EmptyOverrideDoesNotClear(t *testing.T) {
	base := &PromptsConfig{
		MainTaskSystem:         "base sys",
		ReviewFilterTaskSystem: "base filter",
	}
	override := &PromptsConfig{
		MainTaskSystem: "",
	}
	got := MergePrompts(base, override)

	if got.MainTaskSystem != "base sys" {
		t.Errorf("empty override should not clear base: got %q", got.MainTaskSystem)
	}
	if got.ReviewFilterTaskSystem != "base filter" {
		t.Errorf("unset override field should keep base: got %q", got.ReviewFilterTaskSystem)
	}
}

func TestFastTaskSystemPrompt(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	prompt := tpl.FastTaskSystemPrompt()
	if prompt == "" {
		t.Fatal("FastTaskSystemPrompt() returned empty string")
	}
	if !strings.Contains(prompt, "code reviewer") {
		t.Errorf("FastTaskSystemPrompt() should mention code reviewer, got %q", prompt)
	}
}

func TestApplyPromptOverrides_FastTask(t *testing.T) {
	tpl, err := LoadDefault()
	if err != nil {
		t.Fatalf("LoadDefault() error: %v", err)
	}

	cfg := &PromptsConfig{
		FastModeSystem: "custom fast prompt",
	}
	if err := tpl.ApplyPromptOverrides(cfg, "/tmp"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tpl.FastTaskSystemPrompt() != "custom fast prompt" {
		t.Errorf("FastTaskSystemPrompt() = %q, want %q", tpl.FastTaskSystemPrompt(), "custom fast prompt")
	}
}
