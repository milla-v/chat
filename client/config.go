package client

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

// Config is a client config.
type Config struct {
	Address       string `json:"address"`
	User          string `json:"user"`
	Password      string `json:"password"`
	Debug         bool   `json:"debug"`
	SSLSkipVerify bool   `json:"ssl_skip_verify"`
	CacheDir      string `json:"-"`

	configDir  string
	configFile string
}

// NewConfig creates client config
func NewConfig() Config {
	cfg := Config{
		Address:       "wet.voilokov.com:8085",
		SSLSkipVerify: true,
		configDir:     os.Getenv("HOME") + "/.config/chat",
		configFile:    os.Getenv("HOME") + "/.config/chat/chatc.json",
		CacheDir:      os.Getenv("HOME") + "/.cache/chat",
	}

	ensureDirExists(cfg.CacheDir)

	return cfg
}

// Print prints config to stdout.
func (c Config) Print() {
	fmt.Println("cache dir:", c.CacheDir)
	fmt.Println("config file:", c.configFile)
	fmt.Println("config:")
	enc := json.NewEncoder(os.Stdout)
	//	enc.SetIndent("", "    ")
	enc.Encode(&c)
}

// Load loads json config file.
func (c *Config) Load(fname string) {
	c.configFile = fname

	f, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err := dec.Decode(c); err != nil {
		panic(err)
	}
}

// LoadDefault loads default json config file ~/.config/chat/chatc.json.
func (c *Config) LoadDefault() {
	c.configFile = os.Getenv("HOME") + "/.config/chat/chatc.json"
	ensureDirExists(c.configDir)
	c.ensureConfigExists()
	c.Load(c.configFile)
}

func ensureDirExists(dir string) {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0700)
		if err != nil {
			panic(err)
		}
	}
}

func (c *Config) ensureConfigExists() {
	_, err := os.Stat(c.configFile)
	if !os.IsNotExist(err) {
		return
	}

	// create default config file
	buf, err := json.MarshalIndent(c, "    ", "")
	if err != nil {
		panic(err)
	}
	if err = ioutil.WriteFile(c.configFile, buf, 0600); err != nil {
		panic(err)
	}
	panic(c.configFile + " default config file created. Edit it to set credentials")
}
