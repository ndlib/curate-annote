package annote

import (
	"crypto/rand"
	"encoding/base64"
	"log"
	"time"
)

type LimitedToken struct {
	Token   string
	Creator string
	Created time.Time
	Item    string
	Expire  time.Time
	Used    bool
}

func CreateLimitedToken(item string, user string) *LimitedToken {
	tokenbytes := make([]byte, 16)
	_, err := rand.Read(tokenbytes)
	if err != nil {
		log.Println(err)
		return nil
	}

	token := base64.URLEncoding.EncodeToString(tokenbytes)

	now := time.Now()
	expire := now.Add(30 * 24 * time.Hour)

	t := LimitedToken{
		Token:   token,
		Creator: user,
		Created: now,
		Expire:  expire,
		Item:    item,
		Used:    false,
	}

	err = Datasource.SaveTLLToken(t)
	if err != nil {
		log.Println(err)
		return nil
	}

	return &t
}
