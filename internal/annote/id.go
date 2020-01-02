package annote

import (
	"encoding/base64"
	"math/rand"
	"time"
)

var (
	seededrng = false
)

// NewIdentifier returns a unique string suitable for use in URLs and
// filenames. This implementation uses randomness, so there is a
// small possibility of duplicate identifiers being generated.
func NewIdentifier() string {
	if !seededrng {
		rand.Seed(time.Now().Unix())
		seededrng = true
	}
	r := make([]byte, 9) // 9 bytes * (4/3 encoding) = 12 characters
	rand.Read(r)
	s := base64.URLEncoding.EncodeToString(r)
	return s
}
