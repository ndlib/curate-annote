package annote

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
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
	err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(pass))
	if err != nil {
		return ErrPasswordMismatch
	}

	pwcache.Set(username, pass, 24*time.Hour) // keep cached for one day
	return nil
}

func FindUser(username string) *User {
	user, err := Datasource.FindUser(username)
	if err != nil {
		log.Println(err)
		return nil
	}
	return user
}

func FindUserByToken(token string) *User {
	user, err := Datasource.FindUserByToken(token)
	if err != nil {
		log.Println(err)
		return nil
	}
	return user
}

func SaveUser(user *User) error {
	return Datasource.SaveUser(user)
}

func CheckPasswordRecovery(token string) error {
	// get user record and compare with stored hashed password
	user := FindUserByToken(token)
	if user == nil {
		return ErrPasswordMismatch
	}
	return nil
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
