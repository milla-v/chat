// Package config implements chat service config.
package config

import (
	"os"
)

// ServiceConfig is a chat service config.
type ServiceConfig struct {
	Address  string `json:"address"`
	WorkDir  string `json:"work_dir"`
	CertPath string
	Debug    bool `json:"debug"`
}

func hostname() string {
	name, _ := os.Hostname()
	return name
}

// Config is loaded config.
var Config = &ServiceConfig{
	Address:  "wet." + hostname() + ":8085",
	WorkDir:  "/usr/local/www/wet/work/",
	CertPath: "/usr/local/etc/letsencrypt/golang-autocert",
}
