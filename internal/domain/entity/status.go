package entity

// RequirementSeverity - how a status check came out.
type RequirementSeverity int

// The status checks' severities: OK, a warning, an error.
const (
	RequirementOK RequirementSeverity = iota
	RequirementWarning
	RequirementError
)

// Requirement - one line of the status report: what was checked, what was found and, when not OK,
// what to do. Title, Value and Description are already translated;
// Description is HTML (placeholders escaped by i18n), the others text.
type Requirement struct {
	Title       string
	Value       string
	Description string
	Severity    RequirementSeverity
	// Module - the module whose check it is; "" - the core's.
	Module string
}

// IsError - whether the check found an error.
func (r Requirement) IsError() bool { return r.Severity == RequirementError }

// IsWarning - whether the check found something to warn about.
func (r Requirement) IsWarning() bool { return r.Severity == RequirementWarning }

// StatusModule - a module on the status report: its state and the last
// schema migration applied ("" - none).
type StatusModule struct {
	Name          string
	Label         string
	Enabled       bool
	Installed     bool
	SchemaVersion string
}
