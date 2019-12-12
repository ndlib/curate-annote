package annote

import (
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/julienschmidt/httprouter"
)

type profilePage struct {
	Title   string
	User    *User
	Message string
	NewUser *User
}

func ProfileShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	DoTemplate(w, "profile", profilePage{Title: "User Profile", User: user})
}

func ProfileUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	v := profilePage{Title: "Profile", User: user}

	// figure out what is desired...
	cp := r.FormValue("changepass")
	if cp != "" {
		t, err := CreateResetToken(username)
		if err != nil {
			v.Message = err.Error()
			DoTemplate(w, "profile", v)
			return
		}
		http.Redirect(w, r, "/reset?r="+t, 302)
		return
	}

	nu := r.FormValue("newuser")
	if nu != "" {
		newusername := r.FormValue("username")
		newuser, err := CreateNewUser(newusername)
		if err != nil {
			v.Message = err.Error()
		} else {
			v.NewUser = newuser
		}
		DoTemplate(w, "profile", v)
		return
	}

	// no idea. display page again
	DoTemplate(w, "profile", v)
}

func ProfileEditShow(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	username, _, _ := r.BasicAuth()
	user := FindUser(username)

	DoTemplate(w, "profile_edit", profilePage{Title: "Edit Profile", User: user})
}

var (
	orcidRE = regexp.MustCompile(`\d{4}-\d{4}-\d{4}-\d{3}[0-9X]`)
)

func ProfileEditUpdate(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	var err error
	username, _, _ := r.BasicAuth()
	user := FindUser(username)
	v := profilePage{Title: "Profile", User: user}
	var messages []string

	newusername := r.FormValue("username")
	if newusername != username {
		otheruser := FindUser(newusername)
		if otheruser != nil {
			messages = append(messages, "Username already exists")
			goto do_orcid
		}
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
