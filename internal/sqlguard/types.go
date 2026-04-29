// Package sqlguard provides SQL query validation primitives independent of any
// database connection or server state. Its job is to inspect a SQL string and
// decide whether it is safe to execute under a given policy (read-only,
// whitelist, etc.). DB I/O, mutable state, and request-handler concerns stay
// out of this package.
//
// API shape:
//   - Pure validators that don't depend on configuration are exported as free
//     functions (e.g. ValidateStructuralSafety, ValidateUnicodeSafety).
//   - Validators that need policy data (whitelist, read-only flag, allowed
//     databases) are methods on Guard. Construct a Guard with New(Config{...}).
//
// This package is intentionally self-contained: no imports from the parent
// module, no global mutable state beyond pre-compiled regex (which is safe
// for concurrent use). Tests for this package live alongside it.
package sqlguard

// TableRef represents a table or view reference parsed out of a SQL query. The
// Database field is empty for references to the current database.
//
// Table is always lowercased. For references that resolve to a SQL Server
// system schema (sys.*, information_schema.*) the Table field is rewritten to
// "<schema>.<name>" so that whitelist comparisons can deny system-table
// access without needing a separate schema field on every consumer.
type TableRef struct {
	Database string
	Table    string
}

// ObjectType classifies a database object for existence/confirmation checks.
// The string values match SQL Server's sys.objects.type codes used by the
// caller's existence query.
type ObjectType string

const (
	// ObjectTypeTable is a user table.
	ObjectTypeTable ObjectType = "U"
	// ObjectTypeView is a view.
	ObjectTypeView ObjectType = "V"
	// ObjectTypeProcedure is a stored procedure.
	ObjectTypeProcedure ObjectType = "P"
	// ObjectTypeFunction is a scalar function. Callers should also accept
	// the related codes IF, TF, FT when querying sys.objects.
	ObjectTypeFunction ObjectType = "FN"
)

// DDLTarget describes a single object affected by a DDL statement. Returned
// by ExtractDDLTargetObjects so the caller can decide whether a confirmation
// is required (i.e. whether the object actually exists in the database).
type DDLTarget struct {
	Schema  string
	Name    string
	ObjType ObjectType
}

// Logger is the minimal logging surface Guard uses. It mirrors the signature
// of log.Printf and the project's SecurityLogger.Printf so existing loggers
// satisfy it without adapters. Pass nil to silence.
type Logger interface {
	Printf(format string, args ...interface{})
}

// noopLogger is used when Config.Logger is nil. Keeps Guard call sites simple.
type noopLogger struct{}

func (noopLogger) Printf(string, ...interface{}) {}

// Config holds the policy a Guard enforces. Slices are expected to be already
// normalised (lowercased, trimmed) — pass them through ParseWhitelistTables /
// ParseAllowedDatabases or build them yourself.
type Config struct {
	ReadOnly         bool
	Whitelist        []string
	AllowedDatabases []string
	Logger           Logger
}

// Guard enforces a configured policy against incoming SQL queries.
type Guard struct {
	cfg Config
	log Logger
}

// New returns a Guard that applies cfg. Safe for concurrent use.
func New(cfg Config) *Guard {
	g := &Guard{cfg: cfg, log: cfg.Logger}
	if g.log == nil {
		g.log = noopLogger{}
	}
	return g
}

// Whitelist returns the configured whitelist (already normalised). Useful for
// callers that need to inspect the policy without reaching into Config.
func (g *Guard) Whitelist() []string { return g.cfg.Whitelist }

// IsReadOnly reports whether the read-only policy is active.
func (g *Guard) IsReadOnly() bool { return g.cfg.ReadOnly }

// IsAllowedDatabase reports whether db is in the configured cross-database
// allow list. Comparison is exact and case-sensitive — pass already-normalised
// names (typically lowercased) to AllowedDatabases.
func (g *Guard) IsAllowedDatabase(db string) bool {
	for _, allowed := range g.cfg.AllowedDatabases {
		if allowed == db {
			return true
		}
	}
	return false
}
