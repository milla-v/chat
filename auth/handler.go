package auth

import (
	"encoding/base32"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/milla-v/chat/config"
)

var cfg = config.Config

// AuthenticateHandler gets user, password, redirect parameters from request and logs in the user.
// Response has Token session cookie.
// If redirect=1 redirects to /index.html.
func AuthenticateHandler(w http.ResponseWriter, r *http.Request) {
	var user, password, redir string

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "parse form", http.StatusBadRequest)
			return
		}
		user = r.FormValue("user")
		password = r.FormValue("password")
		redir = r.FormValue("redirect")
	} else if r.Method == "GET" {
		user = r.URL.Query().Get("user")
		password = r.URL.Query().Get("password")
		redir = r.URL.Query().Get("redir")
	}

	if user == "" || password == "" {
		http.Error(w, "user or password is empty. Do POST user=USER&password=PASSWD", http.StatusBadRequest)
		return
	}

	ua, err := login(user, password)
	if err != nil {
		http.Error(w, "auth: "+err.Error(), http.StatusUnauthorized)
		return
	}

	expiration := time.Now().Add(365 * 24 * time.Hour)
	cookie := http.Cookie{Name: "token", Value: ua.Token, Expires: expiration}
	http.SetCookie(w, &cookie)

	if redir == "1" {
		http.Redirect(w, r, "/index.html", http.StatusFound)
		return
	}

	// for console clients
	w.Header().Add("Token", ua.Token)
}

// SendEmail sends email to me.
// TODO: move it to utils.
func SendEmail(to, text string) {
	if true {
		fmt.Println(text)
	}

	cmd := exec.Command("sendmail", "-f"+to, cfg.AdminEmail)
	cmd.Stdin = strings.NewReader(text)
	bytes, err := cmd.CombinedOutput()
	log.Println("sendmail:", string(bytes))
	if err != nil {
		log.Println(err)
		return
	}
}

// RegisterHandler gets user and email parameters from request.
// Creates provisional user registration file and sends email to the administrator.
// Email has the link which should call the CreateHandler.
// Administrator should accept the registration by forwarding the email to the user.
// User should follow the link which calls CreateHandler which completes user registration.
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "use POST method to submit user=USER&email=EMAIL", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "parse form", http.StatusBadRequest)
		log.Println(err)
		return
	}

	user := r.FormValue("user")
	email := r.FormValue("email")
	if user == "" || email == "" {
		http.Error(w, "user or email is empty. Do POST user=USER&email=EMAIL", http.StatusBadRequest)
		log.Println("user or email is empty")
		return
	}

	randomString, err := generateRandomString(15)
	if err != nil {
		http.Error(w, "cannot generate registration token", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	registrationToken := base32.StdEncoding.EncodeToString([]byte(randomString))

	fname := cfg.WorkDir + "reg-" + registrationToken + ".txt"
	f, err := os.Create(fname)
	if err != nil {
		http.Error(w, "cannot open registration file", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	fmt.Fprintln(f, user, " ", email)
	f.Close()

	text := "Subject: chat account request\n\n"
	text += "Re: " + user + " " + email + "\n\n"
	text += "To create a new chat account clink the link below\n\n"
	text += "https://" + cfg.Address + "/create?user=" + user + "&email=" + email + "&rt=" + registrationToken + "\n\n"
	text += ".\n"

	SendEmail(email, text)
	fmt.Fprintln(w, "You will receive a confirmation email from administrator.")
}

// CreateHandler checks if user, email and rt parameters match registration file and creates permanent
// user profile. Redirects to AuthenticateHandler with user and password and redirect=1 parameters
func CreateHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	user := q.Get("user")
	email := q.Get("email")
	token := q.Get("rt")
	if user == "" || email == "" || token == "" {
		http.Error(w, "empty parameter", http.StatusBadRequest)
		log.Println("empty parameter")
		return
	}

	fname := cfg.WorkDir + "reg-" + token + ".txt"
	bytes, err := ioutil.ReadFile(fname)
	if err != nil {
		http.Error(w, "cannot read registration file", http.StatusBadRequest)
		log.Println(err)
		return
	}

	fields := strings.Fields(string(bytes))
	if len(fields) < 2 {
		http.Error(w, "invalid registration parameters", http.StatusBadRequest)
		log.Println("wrong fields:", fields)
		return
	}

	if user != fields[0] || email != fields[1] {
		http.Error(w, "invalid registration parameters", http.StatusBadRequest)
		log.Println("wrong fields:", fields, "user:", user, "email:", email)
	}

	randomString, err := generateRandomString(15)
	if err != nil {
		http.Error(w, "cannot generate password", http.StatusInternalServerError)
		log.Println(err)
		return
	}

	password := base32.StdEncoding.EncodeToString([]byte(randomString))

	userFname := cfg.WorkDir + "user-" + user + ".txt"
	uf, err := os.Create(userFname)
	if err != nil {
		http.Error(w, "cannot open registration file", http.StatusInternalServerError)
		log.Println(err)
		return
	}
	fmt.Fprintln(uf, user, password, email)
	uf.Close()

	//fmt.Fprintln(w, "registration completed. Your password is "+password)
	http.Redirect(w, r, "/auth?user="+user+"&password="+password+"&redir=1", http.StatusFound)
}
