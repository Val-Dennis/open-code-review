package tool

import "fmt"

// Tool represents a single review tool.
type Tool struct {
	name string
}

var (
	Unknown    = Tool{name: "unknown"}
	TaskDone   = Tool{name: "task_done"}
	CodeComment = Tool{name: "code_comment"}
	FileRead   = Tool{name: "file_read"}
	FileFind   = Tool{name: "file_find"}
	FileReadDiff = Tool{name: "file_read_diff"}
	CodeSearch  = Tool{name: "code_search"}
)

func OfName(name string) Tool {
	for _, t := range allTools() {
		if t.name == name {
			return t
		}
	}
	return Unknown
}

func allTools() []Tool {
	return []Tool{Unknown, TaskDone, CodeComment, FileRead, FileFind, FileReadDiff, CodeSearch}
}

// Name returns the tool's identifier name.
func (t Tool) Name() string { return t.name }

// IsKnown reports whether the tool is not UNKNOWN.
func (t Tool) IsKnown() bool {
	return t != Unknown
}

// LookupResult holds the result of a single tool lookup.
type LookupResult struct {
	Result     string
	Found      bool
}

// Provider is the interface that all concrete tool implementations satisfy.
// Each tool handles one specific capability (read file, search code, etc.).
type Provider interface {
	// Tool returns which tool this provider implements.
	Tool() Tool
	// Execute runs the tool with the given arguments and returns the result string.
	Execute(args map[string]any) (string, error)
}

// Registry maps tool aliases to their providers. Users register their own implementations here.
type Registry map[string]Provider

// NewRegistry creates an empty registry.
func NewRegistry() Registry {
	return make(Registry)
}

// Register adds a tool provider to the registry.
func (r Registry) Register(p Provider) {
	r[p.Tool().name] = p
}

// Lookup finds a provider by name. Returns a zero-value LookupResult if not found.
func (r Registry) Lookup(name string) LookupResult {
	p, ok := r[name]
	if !ok {
		return LookupResult{Found: false}
	}
	return LookupResult{Result: p.Tool().name, Found: true}
}

// ErrToolNotFound is returned when a tool alias cannot be resolved.
var ErrToolNotFound = fmt.Errorf("tool not found")

// NotAvailableError is the standard message returned when a tool is not registered.
const NotAvailableMsg = "Error: Tool not found. The tool you attempted to call does not exist or is not available. Please check the tool name and try again with a valid tool."
