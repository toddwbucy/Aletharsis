package officeplan

import v4 "github.com/toddwbucy/Aletharsis/internal/evidence/v4"

// Intermediate graphs validate only their outcomes. Complete report validation
// binds retained records after every operation graph has been assembled.
func validatePartialOutcomes(index *v4.Index, records v4.Evidence, trace *v4.TraceIndex) error {
	return index.ValidateOutcomeLinks(records, trace)
}
