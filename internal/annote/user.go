package annote

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"

	cache "github.com/patrickmn/go-cache"
)

type User struct {
	ID             int
	Username       string
	HashedPassword string
	Created        time.Time
	ORCID          string
}

var (
	// Since passwords are checked on (almost) every request, for performance we
	// keep a cache in memory relating user names to passwords. Pairs are cached
	// only when the password already compares correctly to the hashed password in
	// the database.
	//
	// this is kept local to this file so we can change it if needed.
	pwcache *cache.Cache = cache.New(24*time.Hour, 1*time.Hour)

	ErrPasswordMismatch = errors.New("Password does not match")
)

// CheckPassword takes the given user name and password, compares it
// against what is in the database and returns either nil, for a match,
// the error ErrPasswordMismatch if the username/password don't match.
func CheckPassword(username, pass string) error {
	// have we cached this already?
	pw, ok := pwcache.Get(username)
	if ok {
		if pw.(string) == pass {
			// match
			return nil
		}
		// remove from cache on invalid attempt
		pwcache.Delete(username)
		return ErrPasswordMismatch
	}

	// get user record and compare with stored hashed password
	user := FindUser(username)
	if user == nil {
		return ErrPasswordMismatch
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(pass))
	if err != nil {
		return ErrPasswordMismatch
	}

	pwcache.Set(username, pass, 24*time.Hour) // keep cached for one day
	return nil
}

// FindUser returns a user record for the given user name.
// If there is no such user record in the database, nil is returned.
func FindUser(username string) *User {
	user, err := Datasource.FindUser(username)
	if err != nil {
		log.Println(err)
		return nil
	}
	return user
}

// FindUserByToken returns a user record given a password reset token.
// If there is no such token, nil is returned.
func FindUserByToken(token string) *User {
	user, err := Datasource.FindUserByToken(token)
	if err != nil {
		log.Println(err)
		return nil
	}
	return user
}

// SaveUser saves the given user record to the database, possibly updating an
// existing record. User records are identified by their ID number.
func SaveUser(user *User) error {
	return Datasource.SaveUser(user)
}

func ResetPassword(username string, newpass string) error {
	user := FindUser(username)
	if user.ID == 0 { // invalid username
		return nil
	}

	pwcache.Delete(username)

	hp, err := bcrypt.GenerateFromPassword([]byte(newpass), 0)
	if err != nil {
		return err
	}
	user.HashedPassword = string(hp)
	return SaveUser(user)
}

func CreateResetToken(username string) (string, error) {
	user := FindUser(username)
	if user.ID == 0 { // invalid username
		return "", nil
	}

	pwcache.Delete(username)

	tokenbytes := make([]byte, 16)
	_, err := rand.Read(tokenbytes)
	if err != nil {
		return "", err
	}

	user.HashedPassword = base64.URLEncoding.EncodeToString(tokenbytes)

	err = SaveUser(user)
	return user.HashedPassword, err
}

func ClearUserFromCache(username string) {
	pwcache.Delete(username)
}

func CreateNewUser(username string) (*User, error) {
	if username != "" {
		// verify username is not already used
		u := FindUser(username)
		if u != nil {
			return nil, errors.New("Username already exists")
		}
	} else {
		// first find an unused username
		// this has a race condition
		for i := 1; ; i++ {
			username = fmt.Sprintf("tempuser-%04d", i)
			u := FindUser(username)
			if u == nil {
				break
			}
		}
	}

	user := &User{
		Username: username,
		Created:  time.Now(),
	}
	err := SaveUser(user)
	if err != nil {
		return nil, err
	}

	_, err = CreateResetToken(username)
	if err != nil {
		return nil, err
	}

	user = FindUser(username)

	return user, nil
}
