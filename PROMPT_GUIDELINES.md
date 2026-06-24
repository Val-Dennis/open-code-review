# Prompt Customization Guidelines

Open Code Review uses a set of prompt templates to drive its LLM-powered review pipeline. All prompts ship with sensible defaults, but you can override any of them via the user config (`~/.opencodereview/config.json`) or the repo config (`.opencodereview/config.json`).

## Configuration

### Setting prompts inline

Prompts can be set as plain strings in the `"prompts"` section of either config file:

```json
{
  "prompts": {
    "main_task_system": "You are a security-focused code reviewer. Only flag security issues."
  }
}
```

### Setting prompts via file reference

For longer prompts, point to a Markdown file using the `file:` prefix. Paths are resolved relative to the repository root (for repo config) or as absolute paths.

```json
{
  "prompts": {
    "main_task_system": "file:.opencodereview/prompts/main_system.md",
    "plan_task_system": "file:/absolute/path/to/plan_system.md"
  }
}
```

### Using the CLI

```bash
# Inline value
ocr config set prompts.main_task_system "You are a strict code reviewer..."

# File reference
ocr config set prompts.main_task_system "file:my-prompts/system.md"
```

### Priority order

When the same prompt key is set in multiple locations, the following priority applies (highest wins):

1. Repo config (`.opencodereview/config.json`)
2. User config (`~/.opencodereview/config.json`)
3. Built-in defaults

Any prompt key that is not set falls through to the next level. You only need to override the prompts you want to change.

## Available Prompt Keys

Each key corresponds to one message in a specific LLM task. Tasks use a system message (instructions to the LLM) and a user message (the actual review payload).

| Key | Task | Role | Description |
|-----|------|------|-------------|
| `main_task_system` | Main review | system | Core reviewer persona, capabilities, and behavioral rules |
| `main_task_user` | Main review | user | Diff, file context, and review checklist injected per file |
| `plan_task_system` | Plan phase | system | Planning persona for large diffs (tool analysis strategy) |
| `plan_task_user` | Plan phase | user | Diff and context for the planning step |
| `memory_compression_task_system` | Memory compression | system | Instructions for summarizing long conversations |
| `memory_compression_task_user` | Memory compression | user | Conversation history to compress |
| `review_filter_task_system` | Review filter | system | Fact-checker persona for filtering false positives |
| `review_filter_task_user` | Review filter | user | Diff and comments to validate |
| `re_location_task_system` | Re-location | system | Code location assistant for matching comments to lines |
| `re_location_task_user` | Re-location | user | Diff, code snippet, and comment to re-locate |
| `fast_mode_system` | Fast mode (`--fast`) | system | Single-shot reviewer for the `--fast` flag |

## Placeholders

Prompts use `{{placeholder}}` syntax (double curly braces) that get replaced at runtime. When writing custom prompts, you **must** include the required placeholders for that prompt or the review pipeline will not function correctly.

### Main task user (`main_task_user`)

| Placeholder | Required | Description |
|-------------|----------|-------------|
| `{{change_files}}` | Yes | List of other files changed in the same review |
| `{{current_file_path}}` | Yes | Path of the file currently being reviewed |
| `{{diff}}` | Yes | Unified diff of the current file |
| `{{current_system_date_time}}` | No | Current date and time |
| `{{requirement_background}}` | No | User-supplied context from `--background` / `-b` |
| `{{system_rule}}` | Yes | Review checklist rules |
| `{{plan_guidance}}` | No | Output from the plan phase (empty if skipped) |

### Plan task system (`plan_task_system`)

| Placeholder | Required | Description |
|-------------|----------|-------------|
| `{{plan_tools}}` | Yes | Tool descriptions for the planner |

### Plan task user (`plan_task_user`)

| Placeholder | Required | Description |
|-------------|----------|-------------|
| `{{change_files}}` | Yes | List of changed files |
| `{{current_file_path}}` | Yes | Current file path |
| `{{diff}}` | Yes | Unified diff |
| `{{current_system_date_time}}` | No | Current date and time |
| `{{requirement_background}}` | No | User-supplied context |
| `{{system_rule}}` | Yes | Review checklist rules |

### Memory compression task user (`memory_compression_task_user`)

| Placeholder | Required | Description |
|-------------|----------|-------------|
| `{{context}}` | Yes | Serialized conversation history to compress |

### Review filter task user (`review_filter_task_user`)

| Placeholder | Required | Description |
|-------------|----------|-------------|
| `{{path}}` | Yes | File path |
| `{{diff}}` | Yes | Unified diff |
| `{{comments}}` | Yes | Review comments to validate |

### Re-location task user (`re_location_task_user`)

Uses single-brace placeholders:

| Placeholder | Required | Description |
|-------------|----------|-------------|
| `{diff}` | Yes | Unified diff |
| `{existing_code}` | Yes | Original code snippet that failed to match |
| `{suggestion_content}` | Yes | The review comment content |

### System prompts and fast mode

System prompts (`main_task_system`, `plan_task_system`, etc.) and `fast_mode_system` have **no required placeholders**. You are free to rewrite them entirely.

## Writing Effective Custom Prompts

### Do

- **Keep required placeholders** in user prompts. The pipeline substitutes them at runtime; missing placeholders mean missing context.
- **Be specific about the reviewer persona** in system prompts. For example: focus on security, performance, or a specific language's idioms.
- **Test with `--fast` first.** The fast mode system prompt is the easiest to customize since it has no required placeholders.
- **Use file references** for prompts longer than a few sentences. Markdown files are easier to read and maintain than escaped JSON strings.
- **Start from the defaults.** Copy the built-in prompt from `internal/config/template/prompts/` and modify it rather than starting from scratch.

### Don't

- **Don't remove required placeholders** from user prompts. The pipeline will still run, but the LLM will lack context and produce poor results.
- **Don't include tool definitions** in your prompts. Tools are injected separately by the agent.
- **Don't change the output format** of `review_filter_task_user` or `re_location_task_user` unless you also modify the parsing code.
- **Don't use the `file:` prefix for inline strings** that happen to contain the word "file:" at the start. If your inline prompt genuinely starts with "file:", there is no current escape mechanism — use a file reference instead.

## Examples

### Security-focused review

```json
{
  "prompts": {
    "main_task_system": "You are a security-focused code review assistant. Focus exclusively on security vulnerabilities: injection flaws, authentication bypasses, data exposure, insecure deserialization, and cryptographic weaknesses. Ignore code style and non-security concerns."
  }
}
```

### Custom fast mode

```json
{
  "prompts": {
    "fast_mode_system": "You are a senior engineer reviewing a pull request. Be concise. Flag only bugs, security issues, and logic errors. Format each finding as: **[severity]** file:line — description."
  }
}
```

### Using file references for a complete override

```
.opencodereview/
├── config.json
└── prompts/
    ├── main_system.md
    └── main_user.md
```

`.opencodereview/config.json`:
```json
{
  "prompts": {
    "main_task_system": "file:.opencodereview/prompts/main_system.md",
    "main_task_user": "file:.opencodereview/prompts/main_user.md"
  }
}
```
