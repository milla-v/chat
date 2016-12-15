// Package auth implements user registration and authentication for chat project.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"log"
	"strings"
	"github.com/milla-v/chat/config"
)

// User authentication record
type UserAuth struct {
	Name string
	Password string
	Token string    // session token
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
	user = nil
	for _, ua := range list {
		if ua.Token == token {
			user = ua
		}
	}
	
	if user != nil {
		return user, nil
	}
	return nil, errors.New("cannot find user by token")
}

func login(name, password string) (*UserAuth, error) {
	var err error

	for idx, item := range list {
		if item.Name != name || item.Password != password {
			continue
		}
		if len(item.Token) > 0 {
			return list[idx], nil
		}

		list[idx].Token, err = generateRandomString(12)
		if err != nil {
			return nil, errors.New("cannot generate token")
		}

		return list[idx], nil
	}

	fname := config.WorkDir+"user-" + name + ".txt"
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

	token, err := generateRandomString(12)
	if err != nil {
		log.Println(err)
		return nil, errors.New("cannot generate token")
	}

	ua := &UserAuth{
		Name: name,
		Password: password,
		Token: token,
	}

	list = append(list, ua)

	return ua, nil
}
