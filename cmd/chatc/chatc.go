// chatc command runs a simple console chat client.
//
// Usage:
//
//	chatc [flags]
//
// The flags are:
//	-c FILENAME -- specify config file
//	-d          -- enable debug log
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
//	-d FILENAME -- Download file from chat
//
package main

import (
	"flag"
	"io/ioutil"
	"log"
	"os"

	"github.com/milla-v/chat/client"
)

var debug = flag.Bool("debug", false, "log debug output into ~/.cache/chatc/debug.log")
var useConfig = flag.String("c", "", "set config")
var sendFile = flag.String("f", "", "file to send to the chat")
var getFile = flag.String("d", "", "download a file from chat")
var sendText = flag.String("t", "", "text to send to the chat")
var printConfig = flag.Bool("g", false, "print config")

func main() {
	flag.Parse()

	cfg := client.NewConfig()

	if *useConfig != "" {
		cfg.Load(*useConfig)
	} else {
		cfg.LoadDefault()
	}

	if *debug {
		f, err := os.Create(cfg.CacheDir + "/debug.log")
		if err != nil {
			panic(err)
		}
		log.SetOutput(f)
	} else {
		log.SetOutput(ioutil.Discard)
	}

	if *printConfig {
		cfg.Print()
		return
	}

	cli := client.NewClient(cfg)

	if *sendText != "" {
		if err := cli.SendText(*sendText); err != nil {
			panic(err)
		}
		return
	}

	if *sendFile != "" {
		if err := cli.SendFile(*sendFile); err != nil {
			panic(err)
		}
		return
	}

	if *getFile != "" {
		if err := cli.DownloadFile(*getFile); err != nil {
			panic(err)
		}
		return
	}

	log.Println(cli.Listen())
}
