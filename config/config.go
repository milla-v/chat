// Package config implements chat service config.
package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// ServiceConfig is a chat service config.
type ServiceConfig struct {
	Address          string `json:"address"`
	WorkDir          string `json:"work_dir"`
	AdminEmail       string `json:"admin_email"`
	SMTPUser         string `json:"smtp_user"`
	SMTPPasswordFile string `json:"smtp_password_file"`
	PatchDir         string `json:"patch_dir"` // directory for received .patch files
	Debug            bool   `json:"debug"`
}

// Config is loaded config.
var Config = &ServiceConfig{
	Address:    "localhost:8085",
	WorkDir:    os.Getenv("HOME") + "/go/work/",
	AdminEmail: "",
}

var configFile = "/usr/local/etc/chatd.json"

// LoadConfig loads custom or default config.
func LoadConfig(fname string) {
	if fname != "" {
		configFile = fname
	}

	f, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err := dec.Decode(Config); err != nil {
		panic(err)
	}
	log.Println(configFile, "config loaded")
}

// PrintConfig prints loaded config to stdout.
func PrintConfig() {
	fmt.Println("config file:", configFile)
	fmt.Println("loaded config:")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(Config)
}
