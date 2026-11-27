// Package ledger holds the identity and the cycle of delegations: the states
// that block a duplicate and the fingerprint the runner computes from the
// content of the assignment. The deck_* operations arrive in the next layer;
// the structure lives here so the store and the export share it.
package ledger

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// Delegation states. Prepared, launched and returned block a duplicate with the
// same fingerprint; closed releases it.
const (
	StatePrepared = "prepared"
	StateLaunched = "launched"
	StateReturned = "returned"
	StateClosed   = "closed"
)

// Close outcomes.
const (
	OutcomeDone       = "done"
	OutcomeSuperseded = "superseded"
	OutcomeAbsorbed   = "absorbed"
)

// Delegation is one execution of an assignment.
type Delegation struct {
	ID                  string `json:"id"`
	Fingerprint         string `json:"fingerprint"`
	Agent               string `json:"agent"`
	Task                string `json:"task,omitempty"`
	Goal                string `json:"goal,omitempty"`
	State               string `json:"state"`
	Outcome             string `json:"outcome,omitempty"`
	PreparedPhase       int    `json:"prepared_phase,omitempty"`
	OrchestratorSession string `json:"orchestrator_session,omitempty"`
	SubagentSession     string `json:"subagent_session,omitempty"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
	Note                string `json:"note,omitempty"`
}

// Blocking reports whether the state blocks a duplicate fingerprint.
func Blocking(state string) bool {
	return state == StatePrepared || state == StateLaunched || state == StateReturned
}

// NewID returns the identifier of one execution: 12 hex characters from
// crypto/rand. Two legitimate launches of the same assignment never share it.
func NewID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// Normalize trims and collapses whitespace.
func Normalize(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

// shortcut: no Unicode NFC normalization (the standard library cannot do it);
// upgrade if assignments with decomposed text start colliding.
func normalizedJoined(parts []string) string {
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = Normalize(p)
	}
	return strings.Join(out, " | ")
}

// Fingerprint is the sha256 of the normalized content of the assignment:
// agent | task | goal | sorted scope | checks in order | criteria.
func Fingerprint(agent, task, goal string, scope, checks []string, criteria string) string {
	sortedScope := append([]string(nil), scope...)
	sort.Strings(sortedScope)
	text := normalizedJoined([]string{
		agent,
		task,
		goal,
		strings.Join(sortedScope, " | "),
		strings.Join(checks, " | "),
		criteria,
	})
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}
