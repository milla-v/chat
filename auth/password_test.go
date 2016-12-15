package auth

import (
	"encoding/base32"
	"testing"
	"golang.org/x/crypto/scrypt"
)

func TestGenPassword(t *testing.T) {

	rnd64, err := generateRandomBytes(60)
	if err != nil {
		t.FailNow()
	}

	salt := base32.StdEncoding.EncodeToString(rnd64)

	dk, err := scrypt.Key([]byte("some password"), []byte(salt), 16384, 8, 1, 25)
	if err != nil {
		t.FailNow()
	}
	salted_password := base32.StdEncoding.EncodeToString(dk)

	println(salt, len(salt), salted_password, len(salted_password))
}
