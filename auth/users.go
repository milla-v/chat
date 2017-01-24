// Package auth implements user registration and authentication for chat project.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

// UserAuth is a authentication record
type UserAuth struct {
	Name     string
	Password string
	Token    string // session token
}

var list []*UserAuth

func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func generateRandomString(s int) (string, error) {
	b, err := generateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b), err
}

// GetAuthUser finds authenticated user in the list by token.
// TODO: Eventually should accept user id from cookie to minimize user lookup time.
func GetAuthUser(token string) (user *UserAuth, err error) {
	for _, ua := range list {
		if ua.Token == token {
			user = ua
			return
		}
	}

	ua, err := loadUserProfileByToken(token)
	return ua, err
}

func (ua *UserAuth) createToken() error {
	var err error

	ua.Token, err = generateRandomString(12)
	if err != nil {
		log.Println(err)
		return errors.New("cannot generate token: " + err.Error())
	}

	fname := cfg.WorkDir + "token-" + ua.Token + ".txt"
	dateb, _ := time.Now().UTC().MarshalText()
	data := ua.Name + " " + string(dateb)

	err = ioutil.WriteFile(fname, []byte(data), 0600)
	if err != nil {
		log.Println(err)
		return errors.New("cannot write token file: " + err.Error())
	}

	log.Println("user:", ua.Name, "token created:", ua.Token)
	return nil
}

func loadUserProfileByToken(token string) (*UserAuth, error) {
	var err error

	fname := cfg.WorkDir + "token-" + token + ".txt"
	bytes, err := ioutil.ReadFile(fname)
	if err != nil {
		log.Println(err)
		return nil, errors.New("token:" + token + ". cannot read token file: " + err.Error())
	}

	fields := strings.Fields(string(bytes))
	if len(fields) < 2 {
		log.Println(err)
		return nil, errors.New("token:" + token + ". broken token file")
	}

	name := fields[0]
	fname = cfg.WorkDir + "user-" + name + ".txt"
	bytes, err = ioutil.ReadFile(fname)
	if err != nil {
		log.Println(err)
		return nil, errors.New("user:" + name + ". cannot read user profile: " + err.Error())
	}

	fields = strings.Fields(string(bytes))
	if len(fields) < 3 || name != fields[0] {
		log.Println(err)
		return nil, errors.New("user:" + name + ". broken user profile")
	}

	ua := &UserAuth{
		Name:     name,
		Password: fields[1],
		Token:    token,
	}

	log.Println("token:", token, "profile loaded:", ua.Name)
	return ua, nil
}

func loadUserProfileByCredentials(name, password string) (*UserAuth, error) {
	fname := cfg.WorkDir + "user-" + name + ".txt"
	bytes, err := ioutil.ReadFile(fname)
	if err != nil {
		log.Println(err)
		return nil, errors.New("cannot read user profile")
	}

	fields := strings.Fields(string(bytes))
	if len(fields) < 3 {
		log.Println(err)
		return nil, errors.New("broken user profile")
	}

	if name != fields[0] || password != fields[1] {
		log.Println(err)
		return nil, errors.New("wrong user name or password")
	}

	ua := &UserAuth{
		Name:     name,
		Password: password,
	}

	log.Println("name:", name, "profile loaded:", ua.Name)
	return ua, nil
}

func login(name, password string) (*UserAuth, error) {
	var err error

	ua, err := loadUserProfileByCredentials(name, password)
	if err != nil {
		log.Println(err)
		return nil, errors.New("cannot load token: " + err.Error())
	}

	err = ua.createToken()
	if err != nil {
		log.Println(err)
		return nil, errors.New("cannot create token: " + err.Error())
	}

	for idx, item := range list {
		if item.Name == name {
			list[idx] = ua
			return list[idx], nil
		}
	}

	list = append(list, ua)
	return ua, nil
}
