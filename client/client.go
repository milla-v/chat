// Package client implements simple console chat client.
//
package client

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
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

// Client is a console chat client
type Client struct {
	cfg       Config
	ws        *websocket.Conn
	connected bool
	prevRoster string
}

// NewClient creates new client
func NewClient(c Config) Client {
	cli := Client{
		cfg: c,
	}
	return cli
}

func (c *Client) readWebsoket() (*prot.Envelope, error) {
	var e prot.Envelope
	err := websocket.JSON.Receive(c.ws, &e)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (c *Client) processMessage(e prot.Envelope) {

	if e.Message != nil {
		fmt.Printf("%s %s%s%s %s\n",
			e.Message.Ts.Format("15:04"),
			"\x1b["+e.Message.ColorXterm256+"m", e.Message.Name, "\x1b[m",
			e.Message.Text)

		if e.Message.Notification != "" {
			notify(e.Message.Name, e.Message.Notification)
		}
	}

	if e.Ping != nil {
		e.Ping.Pong = e.Ping.Ping
		log.Println("ping:", e.Ping.Ping)
		err := websocket.JSON.Send(c.ws, &e)
		if err != nil {
			fmt.Println(err)
		}
	}

	if e.Roster != nil && e.Roster.Text != c.prevRoster {
		fmt.Printf("%s chatters online: %s\n",
			e.Roster.Ts.Format("15:04"), e.Roster.Text)
			c.prevRoster = e.Roster.Text
	}
}

func (c *Client) newHTTPClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: c.cfg.SSLSkipVerify},
	}
	return &http.Client{Transport: tr}
}

// ErrInvalidCredentials returned when username or password are incorrect.
var ErrInvalidCredentials = errors.New("invalid credentials")

func (c *Client) login() (token string, err error) {
	client := c.newHTTPClient()

	log.Println("connecting to", c.cfg.Address, "as", c.cfg.User)
	vals := url.Values{"user": {c.cfg.User}, "password": {c.cfg.Password}}

	url := "https://" + c.cfg.Address + "/auth"
	log.Println("no token. Get from ", url)

	resp, err := client.PostForm(url, vals)
	if err != nil {
		log.Println("auth error: postform returned:", err)
		return "", err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		log.Println("status:", resp.Status)
		return "", ErrInvalidCredentials
	}

	if resp.StatusCode != http.StatusOK {
		log.Println("status:", resp.Status)
		io.Copy(os.Stderr, resp.Body)
		return "", errors.New("auth error: " + resp.Status)
	}

	token = resp.Header.Get("Token")
	log.Println("auth response. token:", token)
	return token, nil
}

func (c *Client) connect() error {
	token, err := c.login()
	if err != nil {
		return err
	}

	wscfg, err := websocket.NewConfig("wss://"+c.cfg.Address+"/ws", "https://"+c.cfg.Address)
	if err != nil {
		return err
	}

	wscfg.Header.Add("Token", token)
	wscfg.TlsConfig = &tls.Config{
		InsecureSkipVerify: c.cfg.SSLSkipVerify,
	}

	if c.ws, err = websocket.DialConfig(wscfg); err != nil {
		return err
	}

	log.Println("connected")
	return nil
}

// Listen logins into chat and monitors it for incoming messages and prints messages
// to stdout.
func (c *Client) Listen() error {
	log.Println("listen")

	for {
		if c.ws == nil {
			fmt.Printf("%s connecting to %s as %s\n", time.Now().Format("15:04"), c.cfg.Address, c.cfg.User)
			err := c.connect()
			if err == ErrInvalidCredentials {
				fmt.Println(time.Now().Format("15:04"), "error: invalid user name or password")
				log.Fatal(err)
			}
			if err != nil {
				fmt.Println(time.Now().Format("15:04"), "error: cannot connect. Retry in 5 sec")
				log.Println("listen. connect error:", err)
				time.Sleep(time.Second * 5)
				continue
			}
			fmt.Println(time.Now().Format("15:04"), "connected")
		}

		e, err := c.readWebsoket()
		if err != nil {
			fmt.Println(time.Now().Format("15:04"), "disconnected. Reconnect in 5 sec")
			log.Println("listen. read error:", err)
			time.Sleep(time.Second * 5)
			c.ws.Close()
			c.ws = nil
			continue
		}

		c.processMessage(*e)
	}
}

func (c *Client) sendRequest(req *http.Request) error {
	token, err := c.login()
	if err == ErrInvalidCredentials {
		return err
	}

	req.Header.Add("Token", token)

	client := c.newHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		log.Println("send request:", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(os.Stderr, resp.Body)
		log.Println("http Status", resp.Status)
		return errors.New("status: " + resp.Status)
	}

	io.Copy(os.Stdout, resp.Body)
	log.Println("sent successfully")
	return nil
}

// SendText sends a plain text message to the chat.
func (c *Client) SendText(message string) error {
	log.Println("sending text")

	r := strings.NewReader(message)

	req, err := http.NewRequest("POST", "https://"+c.cfg.Address+"/m", r)
	if err != nil {
		return err
	}

	req.Header.Add("ContentType", "text/plain")
	return c.sendRequest(req)
}

// SendFile sends a file to the chat.
func (c *Client) SendFile(fname string) error {
	log.Println("sending file", fname)

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

	url := "https://" + c.cfg.Address + "/upload"
	log.Println("sending to", url)

	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		log.Println("cannot create request to ", url, err)
		return err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	return c.sendRequest(req)
}

// DownloadFile gets a file from the chat.
func (c *Client) DownloadFile(fname string) error {
	url := "https://" + c.cfg.Address + "/" + fname
	log.Println("get file", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Println("cannot create request to ", url, err)
		return err
	}

	token, err := c.login()
	if err == ErrInvalidCredentials {
		return err
	}

	req.Header.Add("Token", token)

	client := c.newHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		log.Println("get file:", err)
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(os.Stderr, resp.Body)
		log.Println("http Status", resp.Status)
		return errors.New("status: " + resp.Status)
	}

	f, err := os.Create(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	written, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	log.Println("downloaded", written, "bytes")
	return nil

}
