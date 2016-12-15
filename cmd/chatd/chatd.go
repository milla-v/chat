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
	"github.com/milla-v/chat/service"
)

var hostport = flag.String("http", "localhost:8085", "chat endpoint")

func main() {
	flag.Parse()
	service.Run(*hostport)
}
