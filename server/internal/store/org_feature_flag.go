package store

// OrgFeatureFlag represents a per-org feature flag with a two-layer model:
// - provisioned: set by platform admins (can the org see this flag at all?)
// - enabled: set by org owner (is the feature active for this org?)
// A feature is only active when BOTH provisioned AND enabled are true.
type OrgFeatureFlag struct {
	Name        string `json:"name"`
	Provisioned bool   `json:"provisioned"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
}
