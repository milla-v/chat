// Package service implements chat http service.
package service

//go:generate go run ../cmd/chatembed/chatembed.go

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/websocket"

	"github.com/milla-v/chat/auth"
	"github.com/milla-v/chat/config"
	"github.com/milla-v/chat/prot"
	"github.com/milla-v/chat/util"
)

type client struct {
	ua              *auth.UserAuth  // user authorization
	ws              *websocket.Conn // websocket connection
	lastMessageTime time.Time       // time of last message
	lastPongTime    time.Time       // time of last pong
	ping            int             // ping number
}

type message struct {
	from  *client
	to    *client
	text  string
	label string
}

var version = "dev"
var date = ""
var clients = []*client{}       // list of active clients (connected and recently disconnected)
var history []prot.Envelope     // recent history for replay to connected client
var recentHistory string        // recent history for emailing to the admin
var connectChan chan *client    // channel to register new client in the list
var connectedChan chan *client  // channel to start client routine
var disconnectChan chan *client // channed to deregister the client
var broadcastChan chan *message // channel to pass message to the worker
var historyFile *os.File        // file for saving all history
var oneMinuteTicker = time.NewTicker(time.Minute)
var certFile = "server.pem"
var keyFile = "server.key"
var cfg = config.Config

func clientRoutine(cli *client) {
	broadcastChan <- &message{cli, nil, "/replay", ""}
	broadcastChan <- &message{cli, nil, "/roster", ""}

	log.Printf("new client: %+v %+v\n", cli, cli.ws)

	for {
		var e prot.Envelope

		err := websocket.JSON.Receive(cli.ws, &e)
		if err != nil {
			log.Printf("disconnecting %s because of %v", cli.ua.Name, err)
			disconnectChan <- cli
			log.Printf("client disconnected")
			break
		}

		log.Printf("ws receive: %s, %+v", cli.ua.Name, e)

		if e.Ping != nil && e.Ping.Ping > 0 {
			if e.Ping.Pong >= e.Ping.Ping {
				log.Printf("pong %s: %d\n", cli.ua.Name, e.Ping.Pong)
				cli.lastPongTime = time.Now()
				broadcastChan <- &message{cli, nil, "/roster", ""}
			}
			continue
		}

		if e.Message != nil {
			cli.lastMessageTime = time.Now()
			cli.lastPongTime = cli.lastMessageTime
			text := html.EscapeString(e.Message.Text)
			broadcastChan <- &message{cli, nil, text, ""}
			continue
		}
	}
}

func removeFromList(cli *client) {
	log.Printf("removing %s", cli.ua.Name)
	for idx, c := range clients {
		log.Printf("ra: %+v,%+v", c.ws.RemoteAddr(), cli.ws.RemoteAddr())
		if c != cli {
			continue
		}
		clients = append(clients[:idx], clients[idx+1:]...)
		cli.ws.Close()
		break
	}
	log.Printf("clients: %d", len(clients))
}

func findClient(token string) (*client, error) {
	for _, c := range clients {
		if c.ua.Token == token {
			return c, nil
		}
	}
	return nil, http.ErrNoCookie
}

func onWebsocketConnection(ws *websocket.Conn) {
	log.Printf("websocket connection: %s", ws.RemoteAddr())
	cli := &client{ws: ws}
	connectChan <- cli
	cli = <-connectedChan
	if cli != nil {
		clientRoutine(cli)
	}
}

var colors = map[string]string{
	"console": "DDFFFF",
	"milla":   "DDFFDD",
	"serge":   "DDDDFF",
}

func replayHistory(cli *client) {
	for _, s := range history {
		err := websocket.JSON.Send(cli.ws, s)
		if err != nil {
			log.Println("send error:", err)
		}
	}
}

func sendToAllClients(from *client, text, label string) {
	e := prot.Envelope{}
	now := time.Now()
	e.Message = new(prot.Message)
	msg := e.Message
	msg.Ts = now
	msg.Name = from.ua.Name
	msg.Text = text
	msg.Notification = label
	msg.Color, _ = colors[strings.ToLower(msg.Name)]
	msg.ColorXterm256 = util.RGB2xterm(msg.Color)

	if label == "" {
		if len(text) > 40 {
			msg.Notification = text[:40] + "..."
		} else {
			msg.Notification = text + " •"
		}
	}

	re := regexp.MustCompile("https?://[^ ]+")
	text = re.ReplaceAllString(text, "<a target=\"chaturls\" href=\"$0\">$0</a>")
	//	id := msg.Ts.Format("m-20060102-150405.000000")

	capname := `<span class="smallcaps">` + strings.Title(from.ua.Name[:3]) + "</span>.\n"
	msg.HTML = "<p>" + capname + text + ` <span class="ts">(` + now.Format("15:04") + ")</span></p>\n"

	//	msg.HTML = fmt.Sprintf("<div id=\"%s\" style=\"background-color: %s\" class=\"msg\">%s %s: <span>%s</span></div>",
	//		id, msg.Color, msg.Ts.Format("15:04"), from.ua.Name, text)

	for _, cli := range clients {
		if cli.ws == nil || from == cli {
			continue
		}

		err := websocket.JSON.Send(cli.ws, &e)
		if err != nil {
			log.Println("cannot send to", cli.ua.Name, err)
		}
	}

	msg.Notification = ""

	if from.ws != nil {
		err := websocket.JSON.Send(from.ws, &e)
		if err != nil {
			log.Println("cannot send to self", from.ua.Name, err)
		}
	}

	msg.HTML = "<p>" + `<span class="ts">` + now.Format("2006-01-02 15:04:05") + "</span> " + msg.Name + ": " + text + "</p>\n"
	recentHistory += msg.HTML + "\n"
	fmt.Fprintln(historyFile, msg.HTML)

	msg.HTML = "<p>" + capname + text + ` <span class="ts">(` + now.Format("15:04") + ")</span></p>\n"
	history = append(history, e)
}

func pingClients() {
	for _, cli := range clients {
		if cli.ws == nil {
			continue
		}

		if time.Since(cli.lastPongTime) < time.Second*60 {
			log.Printf("recent pong: %s\n", cli.ua.Name)
			continue
		}

		if time.Since(cli.lastPongTime) > time.Second*180 {
			log.Printf("no pong for 180 sec, disconnecting %s", cli.ua.Name)
			disconnectChan <- cli
			continue
		}

		cli.ping++
		e := prot.Envelope{}
		e.Ping = new(prot.Ping)
		e.Ping.Timestamp = time.Now()
		e.Ping.Ping = cli.ping

		err := websocket.JSON.Send(cli.ws, &e)
		if err != nil {
			log.Println("send error:", err)
		}
		log.Printf("ping %s\n", cli.ua.Name)
	}
}

func sendRoster(cli *client) {
	if len(clients) == 0 {
		return
	}

	e := prot.Envelope{}
	e.Roster = new(prot.Roster)
	e.Roster.Ts = time.Now()
	text := ""

	for _, cli := range clients {
		text += "<p>" + cli.ua.Name + "</p>\n"
	}

	e.Roster.HTML = text

	log.Printf("sending roster: %+v", e)

	err := websocket.JSON.Send(cli.ws, &e)
	if err != nil {
		log.Println("send error:", err)
	}
}

func broadcastRoster() {
	for _, cli := range clients {
		if cli.ws == nil {
			continue
		}
		sendRoster(cli)
	}
}

func getToken(r *http.Request) (string, error) {
	cookie, err := r.Cookie("token")
	if err == nil {
		return cookie.Value, nil
	}

	token := r.Header.Get("Token")
	if token != "" {
		return token, nil
	}

	return "", errors.New("cannot get token from cookie or header")
}

func connectClient(cli *client) {

	if cli.ws == nil {
		return
	}

	var err error

	token, err := getToken(cli.ws.Request())
	if err != nil {
		log.Println("connect client 1: ", err)
		connectedChan <- nil
		return
	}

	newcli, err := findClient(token)
	if err == nil {
		newcli.ws = cli.ws
		newcli.lastPongTime = time.Now()
		connectedChan <- newcli
		return
	}

	ua, err := auth.GetAuthUser(token)
	if err != nil {
		log.Println("connect client 2:", err)
		connectedChan <- nil
		return
	}

	newcli = &client{ua: ua, ws: cli.ws, lastPongTime: time.Now()}
	clients = append(clients, newcli)
	connectedChan <- newcli
	log.Println("connect client 3.")
}

func emailRecentHistory() {
	if len(recentHistory) == 0 {
		return
	}

	var b bytes.Buffer

	mwr := multipart.NewWriter(&b)

	fmt.Fprintf(&b, "To: %s\n", cfg.AdminEmail)
	fmt.Fprintf(&b, "Subject: chat conversations\n")
	fmt.Fprintf(&b, "Content-Type: multipart/mixed; boundary=%s\n\n", mwr.Boundary())
	headers := make(textproto.MIMEHeader)
	headers.Add("Content-Type", "text/html")
	part, err := mwr.CreatePart(headers)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Fprintln(part, recentHistory)
	fmt.Fprintf(&b, ".\n")
	if len(cfg.AdminEmail) == 0 {
		log.Println("=== recent history ===")
		log.Println(b.String())
	} else {
		auth.SendEmail(cfg.AdminEmail, b.String())
	}
	recentHistory = ""
}

func workerRoutine() {
	for {
		select {
		case <-oneMinuteTicker.C:
			pingClients()
			emailRecentHistory()
		case cli := <-connectChan:
			connectClient(cli)
		case cli := <-disconnectChan:
			removeFromList(cli)
		case msg := <-broadcastChan:
			log.Printf("%+v", msg)
			switch msg.text {
			case "/roster":
				broadcastRoster()
			case "/replay":
				replayHistory(msg.from)
			case "/ping":
				pingClients()
			default:
				sendToAllClients(msg.from, msg.text, msg.label)
			}
		}
	}
}

func generatePage(source, fname string) {
	s := "<!-- This file is generated from files/" + fname + ". Do not edit. -->\n\n" + string(source)
	s = strings.Replace(s, "localhost:8085", cfg.Address, -1)

	s = strings.Replace(s, "{version}", version, 1)
	s = strings.Replace(s, "{date}", date, 1)

	err := ioutil.WriteFile(cfg.WorkDir+fname, []byte(s), 0666)
	if err != nil {
		panic(err)
	}
}

func generatePages() {
	generatePage(indexHTML, "index.html")
	generatePage(loginHTML, "login.html")
}

func messageReceiver(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "only POST method allowed", http.StatusMethodNotAllowed)
		return
	}

	token, err := getToken(r)
	if err != nil {
		http.Error(w, "no token "+err.Error(), http.StatusUnauthorized)
		return
	}

	ua, err := auth.GetAuthUser(token)
	if err != nil {
		http.Error(w, "no auth user", http.StatusUnauthorized)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}
	if len(body) == 0 {
		http.Error(w, "body is empty", http.StatusBadRequest)
		return
	}

	cli, err := findClient(ua.Token)
	if err != nil {
		cli = &client{ua: ua}
	}

	text := html.EscapeString(string(body))
	m := &message{cli, nil, text, ""}
	broadcastChan <- m
}

func createFileServer() http.HandlerFunc {
	generatePages()
	dir := http.Dir(cfg.WorkDir)
	fileserver := http.FileServer(dir)

	f := func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/login.html" {
			cookie, err := r.Cookie("token")
			if err == http.ErrNoCookie {
				http.Redirect(w, r, "/login.html", http.StatusFound)
				return
			}

			_, err = auth.GetAuthUser(cookie.Value)
			if err != nil {
				http.Redirect(w, r, "/login.html", http.StatusFound)
				return
			}
		}

		log.Println("fileserver:", r.URL)
		fileserver.ServeHTTP(w, r)
	}

	return f
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {

	token, err := getToken(r)
	if err != nil {
		http.Error(w, "no token "+err.Error(), http.StatusUnauthorized)
		return
	}

	ua, err := auth.GetAuthUser(token)
	if err != nil {
		http.Error(w, "no auth user", http.StatusUnauthorized)
		return
	}

	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	log.Printf("%+v, %+v", mediaType, params)
	if err != nil {
		http.Error(w, "cannot parse content-type", http.StatusBadRequest)
		return
	}

	mr := multipart.NewReader(r.Body, params["boundary"])

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "cannot get part", http.StatusBadRequest)
			return
		}
		defer part.Close()

		mediaType, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			http.Error(w, "cannot parse part media type", http.StatusBadRequest)
			return
		}
		log.Println(mediaType, params)

		fname := time.Now().Format("20060102150405-") + part.FileName()
		f, err := os.Create(cfg.WorkDir + fname)
		if err != nil {
			http.Error(w, "cannot create file", http.StatusBadRequest)
			return
		}
		defer f.Close()

		var wr io.Writer
		wr = f

		log.Println("ext:", filepath.Ext(fname), ", patch_dir:", cfg.PatchDir)
		if filepath.Ext(fname) == ".patch" && cfg.PatchDir != "" {
			pf, err := os.Create(cfg.PatchDir + fname)
			if err != nil {
				http.Error(w, "cannot create patch file", http.StatusBadRequest)
				return
			}
			wr = io.MultiWriter(f, pf)
			log.Println("=== mw opened")
			defer pf.Close()
		}

		written, err := io.Copy(wr, part)
		if err != nil {
			http.Error(w, "cannot copy file", http.StatusBadRequest)
			return
		}

		fmt.Fprintf(w, "%d bytes sent\n", written)

		text := fmt.Sprintf("file: <a target=\"chaturls\" href=\"%s\">%s</a>", fname, part.FileName())
		m := &message{&client{ua: ua}, nil, text, "file: " + fname}
		broadcastChan <- m
	}
	r.Body.Close()
}

func initWorkDir() {
	_, err := os.Stat(cfg.WorkDir)
	if !os.IsNotExist(err) {
		return
	}

	if err := os.MkdirAll(cfg.WorkDir, 0777); err != nil {
		panic(err)
	}
}

func ensureCertificates() {
	cfname := cfg.WorkDir + certFile
	kfname := cfg.WorkDir + keyFile

	_, err := os.Stat(kfname)
	keyExists := !os.IsNotExist(err)

	_, err = os.Stat(cfname)
	certExists := !os.IsNotExist(err)

	if keyExists && certExists {
		log.Println("key and cert exist")
		return
	}

	log.Println("generating", kfname)
	cmd := exec.Command("openssl", "genrsa", "-out", kfname, "2048")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("generating key: ", string(out))
		panic(err)
	}
	log.Println("key generated")

	log.Println("generating", cfname)
	cmd = exec.Command("openssl", "req", "-new", "-x509", "-sha256",
		"-key", kfname, "-out", cfname,
		"-days", "3650",
		"-subj", "/CN=localhost/C=US/ST=NY/L=NYC/emailAddress="+cfg.AdminEmail)

	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Println("generating cert: ", string(out))
		panic(err)
	}
	log.Println("cert generated")
}

// Run starts a chat http server on address (host:port)
func Run() {
	var err error

	initWorkDir()
	ensureCertificates()

	log.Printf("chat version: %s, date: %s\n", version, date)
	log.Println("starting server on https://" + cfg.Address + "/")

	connectChan = make(chan *client)
	connectedChan = make(chan *client, 100)
	disconnectChan = make(chan *client, 100)
	broadcastChan = make(chan *message, 100)
	historyFile, err = os.OpenFile(cfg.WorkDir+"history.html", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	http.HandleFunc("/", createFileServer())
	http.Handle("/ws", websocket.Handler(onWebsocketConnection))
	http.HandleFunc("/m", messageReceiver)
	http.HandleFunc("/auth", auth.AuthenticateHandler)
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/register", auth.RegisterHandler)
	http.HandleFunc("/create", auth.CreateHandler)

	go workerRoutine()

	err = http.ListenAndServeTLS(cfg.Address, cfg.WorkDir+"server.pem", cfg.WorkDir+"server.key", nil)
	log.Fatal(err)
}
