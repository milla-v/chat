// chatc command runs a simple console chat client.
//
// Usage:
//
//	chatc [flags]
//
// The flags are:
//	-c FILENAME -- Specify config file
//
// Command runs simple chat listener.
//
// Sending messages
//
// To send a message or a file into chat use the same program from other terminal with
// additional flags.
//
// New flags:
//
//	-t "TEXT"   -- Send plain text
//	-f FILENAME -- Send file as an attachment
//
package main

import (
	"flag"
	"github.com/milla-v/chat/client"
)

var useConfig = flag.String("c", "", "Specify config")
var sendFile = flag.String("f", "", "File to send to the chat")
var sendText = flag.String("t", "", "Text to send to the chat")
var printConfig = flag.Bool("g", false, "Print config")

func main() {
	flag.Parse()
	
	if *useConfig != "" {
		client.LoadConfig(*useConfig)
	}

	if *printConfig {
		client.PrintConfig()
		return
	}

	if *sendText != "" {
		if err := client.SendText(*sendText); err != nil {
			panic(err)
		}
	}

	if *sendFile != "" {
		if err := client.SendFile(*sendFile); err != nil {
			panic(err)
		}
	}
	
	if *sendText != "" || *sendFile != "" {
		return
	}

	client.Listen()
}
