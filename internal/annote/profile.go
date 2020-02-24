package annote

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type profilePage struct {
	User    *User
	Message string
	NewUser *User
}

func ProfileShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	DoTemplate(w, "profile", profilePage{User: user})
}

func ProfileUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	v := profilePage{User: user}

	// figure out what is desired...
	cp := r.FormValue("changepass")
	if cp != "" {
		Datasource.RecordEvent("ask-pw-reset", user, "")
		t, err := CreateResetToken(username)
		if err != nil {
			v.Message = err.Error()
			DoTemplate(w, "profile", v)
			return
		}
		http.Redirect(w, r, "/reset?r="+t, 302)
		return
	}

	// no idea. display page again
	DoTemplate(w, "profile", v)
}

func ProfileEditShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	DoTemplate(w, "profile_edit", profilePage{User: user})
}

var (
	orcidRE = regexp.MustCompile(`\d{4}-\d{4}-\d{4}-\d{3}[0-9X]`)
)

func ProfileEditUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var err error
	username, _, _ := r.BasicAuth()
	user := FindUser(username)
	v := profilePage{User: user}
	var messages []string

	newusername := r.FormValue("username")
	if newusername != username {
		otheruser := FindUser(newusername)
		if otheruser != nil {
			messages = append(messages, "Username already exists")
			goto do_orcid
		}
		Datasource.RecordEvent("user-change-username", user, newusername)
		user.Username = newusername
	}

do_orcid:
	orcid := r.FormValue("orcid")
	if orcid != "" {
		neworcid := orcidRE.FindString(orcid)
		if neworcid == "" {
			messages = append(messages, "no orcid in "+orcid)
			goto out
		}
		Datasource.RecordEvent("user-update-orcid", user, "")
		user.ORCID = neworcid
	} else {
		user.ORCID = ""
	}

	err = SaveUser(user)
	if err != nil {
		messages = append(messages, err.Error())
		log.Println(err)
	}
	if newusername != username {
		ClearUserFromCache(username)
	}

out:
	if len(messages) > 0 {
		v.Message = strings.Join(messages, " // ")
		DoTemplate(w, "profile_edit", v)
		return
	}
	http.Redirect(w, r, "/profile", 302)
}

type NewUserData struct {
	Message  string
	Username string
	ORCID    string
}

func ProfileNewShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	DoTemplate(w, "user-new", &NewUserData{})
}

func ProfileNewPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	// make sure data has been entered, if so, try to make new user
	// either return to the sign-up page or redirect to the password reset page

	username := r.FormValue("username")
	orcid := r.FormValue("orcid")
	// prepopulate this data in case there was an error and we want
	// to return it to the user so it can be fixed.
	v := &NewUserData{
		Username: username,
		ORCID:    orcid,
	}
	// set up a deferred func so we can return early from errors
	defer func() {
		if v.Message != "" {
			DoTemplate(w, "user-new", v)
		}
	}()
	if username == "" {
		v.Message = "A user name is required"
		return
	}
	// normalize the ORCID identifier
	if orcid != "" {
		neworcid := orcidRE.FindString(orcid)
		if neworcid == "" {
			v.Message = "Provided ORCID is not well formed"
			return
		}
		orcid = neworcid
	}

	Datasource.RecordEvent("user-create-account", &User{Username: username}, "")

	// try to create this user. CreateNewUser() returns an error
	// if the username already exists.
	user, err := CreateNewUser(username)
	if err != nil {
		log.Println(err)
		v.Message = err.Error()
		return
	}

	if orcid != "" {
		user.ORCID = orcid
		err = SaveUser(user)
		if err != nil {
			log.Println(err)
			v.Message = err.Error()
			return
		}
	}

	http.Redirect(w, r, "/reset?r="+user.HashedPassword, 302)
}
