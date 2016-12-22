package service

import (
	"fmt"
	"os"
	"encoding/json"
	"io/ioutil"
)

type config struct {
	Address string `json:"address"`
	WorkDir string `json:"work_dir"`
}

var cfg = config{
	Address: "localhost:8085",
	WorkDir: "/usr/local/www/wet/work/",
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
		buf, err := json.MarshalIndent(&cfg, "    ", "")
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

func LoadConfig(fname string) {
	configFile = fname

	f, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err := dec.Decode(&cfg); err != nil {
		panic(err)
	}
}

func PrintConfig() {
	fmt.Println("config file:", configFile)
	fmt.Println("loaded config:")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(&cfg)
}
