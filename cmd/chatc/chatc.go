// chatc command runs a simple console chat client.
//
// Usage:
//
//	chatc [flags]
//
// The flags are:
//	-cred=auth~.txt  Specify credentials file in format host:port:user:password
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
//	-t "test"    Send plain text
//	-f file.txt  Send file as an attachment
//
package main

import (
	"flag"
	"github.com/milla-v/chat/client"
)

var sendfile = flag.String("f", "", "File to send to the chat")
var sendtext = flag.String("t", "", "Text to send to the chat")

func main() {
	flag.Parse()
	
	if *sendtext != "" {
		if err := client.SendText(*sendtext); err != nil {
			panic(err)
		}
	}

	if *sendfile != "" {
		if err := client.SendFile(*sendfile); err != nil {
			panic(err)
		}
	}
	
	if *sendtext != "" || *sendfile != "" {
		return
	}

	client.Listen()
}
