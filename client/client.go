// Package client implements simple console chat client.
//
package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/websocket"

	"github.com/milla-v/chat/prot"
)

// Config is a client config.
type Config struct {
	Address       string `json:"address"`
	User          string `json:"user"`
	Password      string `json:"password"`
	Debug         bool   `json:"debug"`
	SSLSkipVerify bool   `json:"ssl_skip_verify"`
}

var cfg = Config{
	Address:       "wet.voilokov.com:8085",
	SSLSkipVerify: true,
}

var configDir = os.Getenv("HOME") + "/.config/chat"
var configFile = configDir + "/chatc.json"
var cacheDir = os.Getenv("HOME") + "/.cache/chat"
var tokenFile = cacheDir + "/token.txt"

var ws *websocket.Conn
var connected = false

func init() {
	_, err := os.Stat(configDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(configDir, 0700)
		if err != nil {
			panic(err)
		}
	}

	_, err = os.Stat(cacheDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(cacheDir, 0700)
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

// LoadConfig loads custom config file.
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

	tokenFile = cacheDir + "/" + cfg.User + "-token.txt"
}

// PrintConfig prints loaded config to stdout.
func PrintConfig() {
	fmt.Println("cache dir:", cacheDir)
	fmt.Println("config file:", configFile)
	fmt.Println("loaded config:")
	enc := json.NewEncoder(os.Stdout)
	//	enc.SetIndent("", "    ")
	enc.Encode(&cfg)
}

func read() (*prot.Envelope, error) {
	var e prot.Envelope
	err := websocket.JSON.Receive(ws, &e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func printMessage(e prot.Envelope) {

	if e.Ping != nil {
		e.Ping.Pong = e.Ping.Ping
		fmt.Printf(".")
		err := websocket.JSON.Send(ws, &e)
		if err != nil {
			fmt.Println(err)
		}
		return
	}

	if e.Message == nil {
		return
	}

	fmt.Printf("%s %s%s%s %s\n",
		e.Message.Ts.Format("15:04"),
		"\x1b["+e.Message.ColorXterm256+"m", e.Message.Name, "\x1b[m",
		e.Message.Text)

	if e.Message.Notification != "" {
		notify(e.Message.Name, e.Message.Notification)
	}
}

func getAuthToken() (string, error) {
	log.Println("read token from", tokenFile)

	_, err := os.Stat(tokenFile)
	if err == nil {
		buf, _ := ioutil.ReadFile(tokenFile)
		token := string(bytes.TrimSpace(buf))
		log.Println("token:", token)
		return token, nil
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.SSLSkipVerify},
	}
	client := &http.Client{Transport: tr}

	vals := url.Values{"user": {cfg.User}, "password": {cfg.Password}}

	url := "https://" + cfg.Address + "/auth"
	log.Println("no token. Get from ", url)

	resp, err := client.PostForm(url, vals)
	if err != nil {
		println("cannot auth")
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		println("status:", resp.Status)
		io.Copy(os.Stderr, resp.Body)
		return "", errors.New("cannot auth")
	}

	token := resp.Header.Get("Token")
	log.Println("token:", token)
	err = ioutil.WriteFile(tokenFile, []byte(token), 0600)
	if err != nil {
		return "", nil
	}
	log.Println("token saved to ", tokenFile)
	return token, err
}

func connect() error {
	token, err := getAuthToken()
	if err != nil {
		os.Remove(tokenFile)
		return err
	}

	config, err := websocket.NewConfig("wss://"+cfg.Address+"/ws", "https://"+cfg.Address)
	if err != nil {
		return err
	}

	config.Header.Add("Token", token)
	config.TlsConfig = &tls.Config{
		InsecureSkipVerify: cfg.SSLSkipVerify,
	}

	ws, err = websocket.DialConfig(config)
	if err != nil {
		return err
	}

	return nil
}

// Listen logs into chat and monitors it for incoming messages and prints messages
// to stdout.
// TODO: replace internal panics with error return value.
func Listen() {
	log.Println("listening")
	os.Remove(tokenFile)

	for {
		if ws == nil {
			if err := connect(); err != nil {
				fmt.Println("connect error:", err)
				time.Sleep(time.Second * 5)
				continue
			}
		}

		e, err := read()
		if err != nil {
			fmt.Println("read error:", err)
			time.Sleep(time.Second * 5)
			continue
		}

		printMessage(*e)
	}
}

// SendText sends a plain text message to the chat.
func SendText(message string) error {
	token, err := getAuthToken()
	if err != nil {
		log.Println("error getting token", err)
		err = os.Remove(tokenFile)
		if err != nil {
			log.Fatal("cannot remove token file", tokenFile, err)
		}
		token, err = getAuthToken()
		if err != nil {
			log.Println("error gettig token2", err)
			return err
		}
	}

	r := strings.NewReader(message)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.SSLSkipVerify},
	}
	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("POST", "https://"+cfg.Address+"/m", r)
	if err != nil {
		return err
	}

	req.Header.Add("ContentType", "text/plain")
	req.Header.Add("Token", token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		log.Println("http StatusUnauthorized. Resending text.")
		err = os.Remove(tokenFile)
		if err != nil {
			log.Fatal("cannot remove token file", tokenFile, err)
		}
		return SendText(message)
	}

	if resp.StatusCode != http.StatusOK {
		io.Copy(os.Stderr, resp.Body)
		return errors.New("status: " + resp.Status)
	}

	io.Copy(os.Stdout, resp.Body)
	return nil
}

// SendFile sends a file to the chat.
func SendFile(fname string) error {
	log.Println("sending file", fname)
	token, err := getAuthToken()
	if err != nil {
		log.Println("error getting token", err)
		err = os.Remove(tokenFile)
		if err != nil {
			log.Fatal("cannot remove token file", tokenFile, err)
		}
		token, err = getAuthToken()
		if err != nil {
			log.Println("error gettig token2", err)
			return err
		}
	}

	var body bytes.Buffer

	f, err := os.Open(fname)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	basename := filepath.Base(fname)
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", basename)
	if err != nil {
		log.Println("cannot create form", err)
		panic(err)
	}

	if _, err = io.Copy(fw, f); err != nil {
		log.Println("cannot copy form", err)
		panic(err)
	}
	w.Close()

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.SSLSkipVerify},
	}
	client := &http.Client{Transport: tr}

	url := "https://" + cfg.Address + "/upload"
	log.Println("sending to", url)

	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		log.Println("cannot create request to ", url, err)
		return err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Token", token)

	resp, err := client.Do(req)
	if err != nil {
		log.Println("cannot send to ", url, err)
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		log.Println("http StatusUnauthorized. Resending file.")
		err = os.Remove(tokenFile)
		if err != nil {
			log.Fatal("cannot remove token file", tokenFile, err)
		}
		return SendFile(fname)
	}

	if resp.StatusCode != http.StatusOK {
		io.Copy(os.Stderr, resp.Body)
		log.Println("http Status", resp.Status)
		return errors.New("status: " + resp.Status)
	}

	io.Copy(os.Stdout, resp.Body)

	log.Println("sent successfully")
	return nil
}
