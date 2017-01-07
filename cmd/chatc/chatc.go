// chatc command runs a simple console chat client.
//
// Usage:
//
//	chatc [flags]
//
// The flags are:
//	-c FILENAME -- specify config file
//  -d          -- output debug info
//
// Command runs simple chat listener.
//
// Sending messages
//
// To send a message or a file into chat use the same program from other terminal with
// additional flags.
//
// Send flags:
//
//	-t "TEXT"   -- Send plain text
//	-f FILENAME -- Send file as an attachment
//
package main

import (
	"flag"
	"io/ioutil"
	"log"

	"github.com/milla-v/chat/client"
)

var debug = flag.Bool("d", false, "output debug info")
var useConfig = flag.String("c", "", "set config")
var sendFile = flag.String("f", "", "file to send to the chat")
var sendText = flag.String("t", "", "text to send to the chat")
var printConfig = flag.Bool("g", false, "print config")

func main() {
	flag.Parse()

	if !*debug {
		log.SetOutput(ioutil.Discard)
	}

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
