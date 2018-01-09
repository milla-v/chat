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

	"github.com/milla-v/chat/config"
	"github.com/milla-v/chat/service"
)

var useConfig = flag.String("c", "", "Specify config file")
var printConfig = flag.Bool("g", false, "Print config file")
var printVersion = flag.Bool("version", false, "Print version")

// Version is set by linker
var Version string

func main() {
	flag.Parse()
	if *printVersion {
		fmt.Println("version:", Version)
		return
	}

	config.LoadConfig(*useConfig)

	if *printConfig {
		config.PrintConfig()
		return
	}

	service.Run()
}
