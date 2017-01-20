package client

import (
	"log"
	"os/exec"
)

func notify(title, text string) {
	err := exec.Command("terminal-notifier",
		"-group", "chatc",
		"-title", "chatc",
		"-subtitle", title,
		"-sound", "default",
		"-message", text).Run()
	if err != nil {
		log.Println("cannot call terminal-notifier", err)
	}
}
