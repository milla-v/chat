// chatembed command generates embedded_files.go file from files directory.
//
// Usage:
//	cd chat/servive
//	go generate
//
package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

func main() {
	main, err := ioutil.ReadFile("../files/index.html")
	if err != nil {
		panic(err)
	}

	toc, err := ioutil.ReadFile("../files/login.html")
	if err != nil {
		panic(err)
	}

	f, err := os.Create("embedded_files.go")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	fmt.Fprintln(f, "package service")
	fmt.Fprintln(f, "const (")
	fmt.Fprintf(f, "indexHTML = `%s`\n\n", string(main))
	fmt.Fprintf(f, "loginHTML = `%s`\n\n", string(toc))
	fmt.Fprintln(f, ")")
	fmt.Println("generated")
}
