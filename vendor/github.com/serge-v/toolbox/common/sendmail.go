package common

import (
	"bytes"
	"log"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// GetRcVar gets variable from rc.subr.ocean file.
func GetRcVar(name string) string {
	cmd := exec.Command("sh", "-c", ". /usr/local/etc/rc.subr.ocean; echo ${"+name+"}")
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

// Sendmail sends mail using system sendmail
func Sendmail(to string, b []byte) error {
	sendmail := exec.Command("/usr/sbin/sendmail", to)
	sendmail.Stdin = bytes.NewReader(b)
	out, err := sendmail.CombinedOutput()
	log.Println(string(out))
	if err != nil {
		return errors.Wrap(err, "sendmail error")
	}
	return nil
}
