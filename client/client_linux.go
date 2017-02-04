package client

import (
	"log"
	"os/exec"
)

func notify(title, text string) {
	err := exec.Command("notify-send", "chatc", text).Run()
	if err != nil {
		log.Println("notify-send", err)
	}
}
