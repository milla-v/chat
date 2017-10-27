package client

import (
	"log"
	"os/exec"
)

func notify(title, text string) {
	err := exec.Command("notify-send", "-t", "60000", "chatc", text).Run()
	if err != nil {
		log.Println("notify-send", err)
	}
}
