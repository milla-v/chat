// Package config implements chat service config.
package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// ServiceConfig is a chat service config.
type ServiceConfig struct {
	Address          string `json:"address"`
	WorkDir          string `json:"work_dir"`
	AdminEmail       string `json:"admin_email"`
	SMTPUser         string `json:"smtp_user"`
	SMTPPasswordFile string `json:"smtp_password_file"`
}

// Config is loaded config.
var Config = &ServiceConfig{
	Address:    "localhost:8085",
	WorkDir:    "/usr/local/www/wet/work/",
	AdminEmail: "",
}

var configDir = os.Getenv("HOME") + "/.config/chat"
var configFile = configDir + "/chatd.json"

func init() {
	_, err := os.Stat(configDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(configDir, 0700)
		if err != nil {
			panic(err)
		}
	}

	_, err = os.Stat(configFile)
	if os.IsNotExist(err) {
		buf, err := json.MarshalIndent(Config, "    ", "")
		if err != nil {
			panic(err)
		}
		if err = ioutil.WriteFile(configFile, buf, 0600); err != nil {
			panic(err)
		}
		panic(configFile + " config file created. Edit it to set credentials")
	}

	LoadConfig(configFile)
}

// LoadConfig loads custom config.
func LoadConfig(fname string) {
	configFile = fname

	f, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err := dec.Decode(Config); err != nil {
		panic(err)
	}
}

// PrintConfig prints loaded config to stdout.
func PrintConfig() {
	fmt.Println("config file:", configFile)
	fmt.Println("loaded config:")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(Config)
}
