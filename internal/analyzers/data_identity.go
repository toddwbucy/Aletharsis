package analyzers

import "github.com/toddwbucy/Aletharsis/internal/evidence"

// EmbeddedDataDigests identifies the exact compiled data files, never a runtime
// filesystem path. Returned maps are independent per call.
func EmbeddedDataDigests() (map[string]string, error) {
	hashes := map[string]string{}
	for _, name := range []string{"emoji-17.0.txt", "messages.json", "limitations.json"} {
		raw, err := data.ReadFile("data/" + name)
		if err != nil {
			return nil, err
		}
		hashes[name] = evidence.Hash(raw)
	}
	return hashes, nil
}
