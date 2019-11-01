package annote

import (
	"time"
)

type User struct {
	Username string
	Passhash string
	Created  time.Time
	ORCID    string
}
