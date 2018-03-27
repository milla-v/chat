// chatd command runs a chatd server.
//
// Usage:
//
//	chatd -http host:port
//
// Command runs standalone server from chat/service package.
//
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/NYTimes/logrotate"
	"github.com/milla-v/chat/config"
	"github.com/milla-v/chat/service"
)

var useConfig = flag.String("c", "", "Specify config file")
var printConfig = flag.Bool("g", false, "Print config file")
var version = flag.Bool("version", false, "Print version")
var daemon = flag.Bool("daemon", false, "Run as a daemon")

// Version is set by linker
var Version string

func main() {
	flag.Parse()
	if *version {
		fmt.Println("version:", Version)
		return
	}
	flag.Parse()
	if *version {
		fmt.Println(Version)
		return
	}
	if *daemon {
		logfile, err := logrotate.NewFile("/var/log/chat.log")
		if err != nil {
			log.Fatal(err)
		}
		log.SetOutput(logfile)
		defer logfile.Close()
		logwriter = logfile
	}

	config.LoadConfig(*useConfig)

	if *printConfig {
		config.PrintConfig()
		return
	}

	service.Run()
}
