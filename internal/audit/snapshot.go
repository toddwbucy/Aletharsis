package audit

// InspectSnapshot retains the exact acquired bytes for downstream presentation.
// It performs the same single, bounded, no-atime acquisition as Inspect. The
// caller owns the returned buffer; failures do not expose partial source bytes.
func InspectSnapshot(path string, limit int) (Outcome, []byte) {
	return inspectSnapshotWithReader(path, limit, readSnapshot)
}
func inspectSnapshotWithReader(path string, limit int, read func(string, int) ([]byte, error)) (Outcome, []byte) {
	var source []byte
	outcome := inspectWithReader(path, limit, func(path string, limit int) ([]byte, error) {
		data, err := read(path, limit)
		if err == nil {
			source = data
		}
		return data, err
	})
	if outcome.Failure != nil {
		return outcome, nil
	}
	return outcome, source
}
