package schemas

import _ "embed"

//go:embed profile-v1.schema.json
var profile1 string

// Profile returns the inert PA-001 profile definition contract. This is not a
// report variant and does not enable profile assessments in existing reports.
func Profile() []byte { return []byte(profile1) }
