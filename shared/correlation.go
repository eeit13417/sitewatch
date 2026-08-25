package shared

import (
	"crypto/rand"
	"encoding/hex"
)

// NewCorrelationID returns a short random hex string — not a UUID, and
// deliberately so: nothing here needs UUID's structure (version bits,
// global-registry semantics), just a cheap, collision-unlikely tag to grep
// one unit of work's log lines by. Adding a UUID dependency for this would
// be pulling in a library to do less than crypto/rand already does.
func NewCorrelationID() string {
	buf := make([]byte, 8) // 16 hex chars — plenty for a log-grep key, not a security token
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read only fails if the OS RNG is unavailable, which
		// means far bigger problems than a missing correlation id — never
		// block message/request processing over it.
		return "unavailable"
	}
	return hex.EncodeToString(buf)
}
