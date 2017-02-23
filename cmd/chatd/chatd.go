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

	"github.com/milla-v/chat/config"
	"github.com/milla-v/chat/service"
)

var useConfig = flag.String("c", "", "Specify config file")
var printConfig = flag.Bool("g", false, "Print config file")
var printVersion = flag.Bool("v", false, "Print version")

func main() {
	flag.Parse()
	if *printVersion {
		service.PrintVersion()
		return
	}

	config.LoadConfig(*useConfig)

	if *printConfig {
		config.PrintConfig()
		return
	}

	service.Run()
}

