// Package personas defines the Aegis agent persona system.
//
// Each persona targets a different security research domain:
//
//   - sharingan: 👁️ Full security audit & reconnaissance (Naruto)
//   - senku:     🧪 Dependency & supply chain analysis (Dr. Stone)
//   - killua:    ⚡ Targeted penetration testing (Hunter × Hunter)
//
// Personas inject into the system prompt's Tier 2 (user content) via
// StructuredPrompt — they augment the agent's default identity rather than
// replacing it.
package personas

import "github.com/pixelvide/localharness/adk"

// Persona defines the contract for an agent persona.
type Persona interface {
	// Name returns the unique identifier (e.g., "sharingan", "senku").
	Name() string

	// Description returns a short human-readable summary.
	Description() string

	// Prompt returns the StructuredPrompt for this persona.
	// Identity MUST be empty — personas augment the default identity.
	Prompt() *adk.StructuredPrompt

	// DefaultMessage returns the default chat message to kick off the agent.
	DefaultMessage() string

	// JournalFile returns the filename for this persona's journal
	// (e.g., "sharingan.md"). Stored under .aegis/ in the workspace root.
	JournalFile() string

	// SupportsPipeline returns true if this persona supports the parallel
	// scan pipeline (chunking → parallel scanning → dedup → review).
	// When true, main.go routes the scan through the pipeline engine
	// instead of the legacy single-chat flow.
	SupportsPipeline() bool
}

// registry holds all registered personas, keyed by Name().
var registry = map[string]Persona{}

// Register adds a persona to the global registry.
func Register(p Persona) {
	registry[p.Name()] = p
}

// Get returns a persona by name and whether it was found.
func Get(name string) (Persona, bool) {
	p, ok := registry[name]
	return p, ok
}

// All returns a copy of the full registry map.
func All() map[string]Persona {
	out := make(map[string]Persona, len(registry))
	for k, v := range registry {
		out[k] = v
	}
	return out
}
