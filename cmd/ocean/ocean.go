// ocean command deploys a program using ssh to digitalocean cloud server.
//
// Usage:
//	ocean [flags]
//
// Flags:
//	-deploy
//		deploy program. (Runs ./mkdist.sh locally; copies and unpacks package; runs configure.sh remotely)
//	-status
//		returns status of remote server
//	-g
//		dump config
//
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	deploy      = flag.Bool("deploy", false, "deploy program")
	status      = flag.Bool("status", false, "show remote server status")
	printConfig = flag.Bool("g", false, "print config")

	client *ssh.Client
)

type config struct {
	PrivateKeyFile string `json:"pk_file"`
	Address        string `json:"address"`
	User           string `json:"user"`
	privateKey     []byte
}

var configDir = os.Getenv("HOME") + "/.config/ocean/"
var configFile = configDir + "ocean.json"

var cfg = config{
	PrivateKeyFile: configDir + "id_rsa_ocean",
}

func dumpConfig() {
	fmt.Println("config file:", configFile)
	fmt.Println("loaded config:")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "    ")
	enc.Encode(&cfg)
}

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
		var buf []byte
		buf, err = json.MarshalIndent(&cfg, "    ", "")
		if err != nil {
			panic(err)
		}
		if err = ioutil.WriteFile(configFile, buf, 0600); err != nil {
			panic(err)
		}
		panic(configFile + " config file created. Edit it to set credentials")
	}

	f, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	if err = dec.Decode(&cfg); err != nil {
		panic(err)
	}

	if cfg.privateKey, err = ioutil.ReadFile(configDir + cfg.PrivateKeyFile); err != nil {
		panic(err)
	}
}

func createSSHClient() *ssh.Client {
	signer, err := ssh.ParsePrivateKey([]byte(cfg.privateKey))
	if err != nil {
		panic(err)
	}

	config := &ssh.ClientConfig{
		User:    cfg.User,
		Auth:    []ssh.AuthMethod{ssh.PublicKeys(signer)},
		Timeout: time.Second * 3,
	}

	client, err = ssh.Dial("tcp", cfg.Address, config)
	if err != nil {
		panic(err)
	}
	return client
}

func createSession() *ssh.Session {
	session, err := client.NewSession()
	if err != nil {
		panic(err)
	}

	session.Stdout = os.Stdout
	session.Stdout = os.Stderr

	return session
}

func deployPackage(fname string) {
	session := createSession()

	// this function does the same as the following commands
	// cat file | ssh host 'cat > file'

	f, err := os.Open(fname)
	if err != nil {
		panic(err)
	}

	// connect ssh session input to the opened file
	session.Stdin = f

	fmt.Printf("copying %s to the ocean\n", fname)
	cmd := "cat > " + fname
	if err := session.Run(cmd); err != nil {
		panic(err)
	}

	session.Close()
	session = createSession()
	defer session.Close()

	// run remote deploy commands
	fmt.Println("deploying " + fname)
	cmd = "sudo tar -C / -xzf " + fname + "; sudo /usr/local/lib/" + fname + "-configure.sh"
	if err := session.Run(cmd); err != nil {
		panic(err)
	}
	fmt.Println("done")
}

func getStatus() {
	session := createSession()
	defer session.Close()
	if err := session.Run("uname -a"); err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()
	if flag.NFlag() == 0 {
		flag.Usage()
		return
	}

	if *printConfig {
		dumpConfig()
		return
	}

	client = createSSHClient()
	defer client.Close()

	if *deploy {
		cmd := exec.Command("./mkdist.sh")
		out, err := cmd.CombinedOutput()
		fmt.Println(string(out))
		if err != nil {
			panic(err)
		}
		deployPackage("chat.tar.gz")
		fmt.Println("deployment done")
		return
	}

	if *status {
		getStatus()
		return
	}
}
