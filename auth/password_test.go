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
	saltedPassword := base32.StdEncoding.EncodeToString(dk)

	println(len(rnd64), salt, len(salt), saltedPassword, len(saltedPassword))
}
