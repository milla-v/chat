// ocean command deploys a program using ssh to digitalocean cloud server.
// It searches input file for a deployment rules and applies them to the cloud server.
// You can customize rules and params to deploy to any ssh capable server.
//
// Usage:
//	ocean [flags]
//
// Flags:
//	-def                  Execute predefined rules (mkdist.sh; scp; ssh configure)
//	-deploy rule_file.go  Processes rules from input file
//	-status               Returns status of remote server
//	-tty                  Starts basic tty session on remote server
//
// Rules:
//	// pkg-build script.sh    Run script locally to build deployment package
//	// pkg-deploy pkg.tar.gz  Copy package to the cloud, untar, run pkg-configure script
//
// Rules should contain comment "//" characters at the beggining of the line.
//
// In order to build this program you need to provide your sshParams for connecting to remote server.
// Add your params to ocean.go
//
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	def    = flag.Bool("def", false, "Execute predefined rules")
	deploy = flag.String("deploy", "", "Rules file")
	status = flag.Bool("status", false, "Show server server status")
	tty    = flag.Bool("tty", false, "Open tty session")

	client *ssh.Client
)

type config struct {
	PrivateKeyFile string `json:"pk_file"`
	Address        string `json:"address"`
	User           string `json:"user"`
	privateKey     []byte
}

var cfg = config{}

var configDir = os.Getenv("HOME") + "/.config/ocean/"
var configFile = configDir + "ocean.json"

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

func deployFile(fname string) {
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

func runTerminal() {
	session := createSession()
	defer session.Close()

	in, err := session.StdinPipe()
	if err != nil {
		panic(err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	// Request pseudo terminal
	if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
		panic(err)
	}

	// Start remote shell
	if err := session.Shell(); err != nil {
		panic(err)
	}

	for {
		reader := bufio.NewReader(os.Stdin)
		str, err := reader.ReadString('\n')
		if err != nil {
			println(err)
			break
		}
		fmt.Fprint(in, str)
	}
}

func getStatus() {
	session := createSession()
	defer session.Close()
	if err := session.Run("ls -l /usr/local/www/wet/"); err != nil {
		panic(err)
	}
}

func execInstallRules(fname string) {
	f, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "// pkg-build ") {
			pars := strings.SplitN(s, " ", 3)
			if len(pars) != 3 {
				panic("invalid pkg-build command " + s)
			}

			cmd := exec.Command(pars[2])
			out, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Println("errors:[\n", string(out), "]")
				panic(err)
			}
			fmt.Println(string(out))
		} else if strings.HasPrefix(s, "// pkg-deploy ") {
			pars := strings.SplitN(s, " ", 3)
			if len(pars) != 3 {
				panic("invalid pkg-build command " + s)
			}
			deployFile(pars[2])
			fmt.Println("deployment done")
			break
		}
	}

	if err = scanner.Err(); err != nil {
		panic(err)
	}
}

func main() {
	flag.Parse()
	if flag.NFlag() == 0 {
		flag.Usage()
		return
	}

	client = createSSHClient()
	defer client.Close()

	if *deploy != "" {
		execInstallRules(*deploy)
	} else if *def {
		cmd := exec.Command("./mkdist.sh")
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println("errors:[\n", string(out), "]")
			panic(err)
		}
		fmt.Println(string(out))
		deployFile("chat.tar.gz")
		fmt.Println("deployment done")
	} else if *status {
		getStatus()
	} else if *tty {
		runTerminal()
	}
}
