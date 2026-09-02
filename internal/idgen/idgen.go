package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// New returns a prefixed random ID, e.g. New("prod") -> "prod_3a9f...".
// Prefixes make IDs self-describing in logs, URLs, and support tickets.
func New(prefix string) string {
	return fmt.Sprintf("%s_%s", prefix, randomHex(12))
}

// APIKey returns a merchant API key. Swap the prefix per environment
// (sk_live_ / sk_test_) if you add a sandbox mode later.
func APIKey() string {
	return fmt.Sprintf("sk_%s", randomHex(24))
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing means the OS entropy source is broken;
		// there's nothing sane to fall back to.
		panic(fmt.Sprintf("idgen: crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}
