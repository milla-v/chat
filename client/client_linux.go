package client

import (
	"log"
	"os/exec"
)

func notify(text string) {
	err := exec.Command("notify-send", "chatc", text).Run()
	if err != nil {
		log.Println("notify-send", err)
	}
}
