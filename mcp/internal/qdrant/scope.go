package qdrant

const (
	// ScopeGlobal represents memories accessible from any project/user
	ScopeGlobal = "global"
	// ScopePersonal represents memories accessible only by the current user
	ScopePersonal = "personal"
	// ScopeProject represents memories accessible only within a specific project
	ScopeProject = "project"
)

// Vector names for multi-vector embeddings
const (
	VectorMain     = "main"
	VectorProject  = "project"
	VectorContext  = "context"
	VectorContent  = "content"
	VectorScope    = "scope"
	VectorKeyword1 = "keyword1"
	VectorKeyword2 = "keyword2"
	VectorKeyword3 = "keyword3"
	VectorKeyword4 = "keyword4"
	VectorKeyword5 = "keyword5"
)

// ValidScope returns true if the given scope is valid
func ValidScope(scope string) bool {
	return scope == ScopeGlobal || scope == ScopePersonal || scope == ScopeProject
}

// DefaultScope returns the default scope (project) for backward compatibility
func DefaultScope() string {
	return ScopeProject
}
