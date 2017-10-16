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
	"unicode/utf8"

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

var (
	version          string
	date             string
	clients          = []*client{}   // list of active clients (connected and recently disconnected)
	history          []prot.Envelope // recent history for replay to connected client
	recentHistory    string          // recent history for emailing to the admin
	connectChan      chan *client    // channel to register new client in the list
	connectedChan    chan *client    // channel to start client routine
	disconnectChan   chan *client    // channed to deregister the client
	broadcastChan    chan *message   // channel to pass message to the worker
	historyFile      *os.File        // file for saving all history
	tenMinutesTicker = time.NewTicker(time.Minute * time.Duration(10))
	certFile         = "server.pem"
	keyFile          = "server.key"
	cfg              = config.Config
)

const helpText = `
	/help   &mdash; print this help
	/roster &mdash; refresh user list
	f       &mdash; show/hide file send panel
	n       &mdash; show/hide notifications
	.       &mdash; answer да
	!       &mdash; answer ДА!!!
	,       &mdash; answer нет
	`

// PrintVersion prints service version to the stdout.
func PrintVersion() {
	fmt.Println("version:", version)
	fmt.Println("date:   ", date)
}

func clientRoutine(cli *client) {
	broadcastChan <- &message{cli, nil, "/replay", ""}
	broadcastChan <- &message{cli, nil, "/roster", ""}

	for {
		var e prot.Envelope

		err := websocket.JSON.Receive(cli.ws, &e)
		if err != nil {
			log.Printf("disconnecting %s because of %v", cli.ua.Name, err)
			disconnectChan <- cli
			log.Printf("client disconnected")
			break
		}

		if e.Ping != nil && e.Ping.Ping > 0 {
			if e.Ping.Pong >= e.Ping.Ping {
				log.Printf("ws pong. user: %s, pong: %d", cli.ua.Name, e.Ping.Pong)
				cli.lastPongTime = time.Now()
				broadcastChan <- &message{cli, nil, "/roster", ""}
			}
			continue
		}

		if e.Message != nil {
			log.Printf("ws msg. user: %s, text: %s", cli.ua.Name, e.Message.Text)
			cli.lastMessageTime = time.Now()
			cli.lastPongTime = cli.lastMessageTime
			text := html.EscapeString(strings.TrimSpace(e.Message.Text))
			broadcastChan <- &message{cli, nil, text, ""}
			continue
		}

		log.Printf("ws unknown. user: %s: %+v", cli.ua.Name, e)
	}
}

func removeFromList(cli *client) {
	log.Println("removing", cli.ua.Name, "remoteAddr:", cli.ws.Request().RemoteAddr)
	for idx, c := range clients {
		if c != cli {
			continue
		}
		clients = append(clients[:idx], clients[idx+1:]...)
		cli.ws.Close()
		break
	}
	log.Printf("clients left: %d", len(clients))
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

func cutRunes(s string, n int) string {
	if n > utf8.RuneCountInString(s) {
		return s + " •"
	}

	cutpos := 0
	count := 0
	for pos, rune := range s {
		count++
		if count >= n {
			cutpos = pos + utf8.RuneLen(rune)
			break
		}
	}

	return s[:cutpos] + "..."
}

func autoreplaceText(s string) string {
	switch s {
	case ".":
		return "да."
	case ",":
		return "нет."
	case "!":
		return "ДА!!!"
	}
	return s
}

func sendToAllClients(from *client, text, label string) {
	e := prot.Envelope{}
	now := time.Now()
	e.Message = new(prot.Message)
	msg := e.Message
	msg.Ts = now
	msg.Name = from.ua.Name
	msg.Text = autoreplaceText(text)
	msg.Notification = label
	msg.Color, _ = colors[strings.ToLower(msg.Name)]
	msg.ColorXterm256 = util.RGB2xterm(msg.Color)

	if label == "" {
		msg.Notification = cutRunes(text, 64)
	}

	re := regexp.MustCompile("https?://[^ \n]+")
	text = re.ReplaceAllString(text, "<a target=\"chaturls\" href=\"$0\">$0</a>")
	if strings.Contains(text, "\n") {
		text = "<pre>" + msg.Text + "</pre>"
	}
	capname := `<span class="smallcaps">` + strings.Title(from.ua.Name[:3]) + "</span>.\n"
	msg.HTML = "<p>" + capname + msg.Text + ` <span class="ts">(` + now.Format("15:04") + ")</span></p>\n"

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

		if time.Since(cli.lastPongTime) < time.Second*480 {
			log.Printf("recent pong: %s\n", cli.ua.Name)
			continue
		}

		if time.Since(cli.lastPongTime) > time.Second*720 {
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

func sendHelp(cli *client) {
	if len(clients) == 0 {
		return
	}

	e := prot.Envelope{}
	e.Message = new(prot.Message)
	e.Message.Ts = time.Now()

	e.Message.Text = helpText
	e.Message.HTML = "<p><pre>" + e.Message.Text + "</pre></p>\n"

	err := websocket.JSON.Send(cli.ws, &e)
	if err != nil {
		log.Println("send error:", err)
	}
}

func sendRoster(cli *client) {
	e := prot.Envelope{}
	e.Roster = new(prot.Roster)
	now := time.Now()
	e.Roster.Ts = now

	for _, cli := range clients {
		e.Roster.Text += cli.ua.Name + ", "
	}

	e.Roster.Text = strings.Trim(e.Roster.Text, ", ")
	e.Roster.HTML = "in room: " + e.Roster.Text

	log.Printf("sending roster: %s", e.Roster.Text)

	err := websocket.JSON.Send(cli.ws, &e)
	if err != nil {
		log.Println("send error:", err)
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

	log.Printf("websocket connection. remote addr: %s", cli.ws.Request().RemoteAddr)

	token, err := getToken(cli.ws.Request())
	if err != nil {
		log.Println("connect client. get token error: ", err)
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
		log.Println("connect client. get auth user error:", err)
		connectedChan <- nil
		return
	}

	newcli = &client{ua: ua, ws: cli.ws, lastPongTime: time.Now()}
	clients = append(clients, newcli)
	connectedChan <- newcli
	log.Println("connect client. connected:", ua.Name)
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
		case <-tenMinutesTicker.C:
			pingClients()
			emailRecentHistory()
		case cli := <-connectChan:
			connectClient(cli)
		case cli := <-disconnectChan:
			removeFromList(cli)
		case msg := <-broadcastChan:
			// log.Printf("%+v", msg)
			switch msg.text {
			case "/roster":
				sendRoster(msg.from)
			case "/help":
				sendHelp(msg.from)
			case "/replay":
				replayHistory(msg.from)
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
		log.Println("receiver: no POST method")
		return
	}

	token, err := getToken(r)
	if err != nil {
		http.Error(w, "no token "+err.Error(), http.StatusUnauthorized)
		log.Println("receiver: no token.", err)
		return
	}

	ua, err := auth.GetAuthUser(token)
	if err != nil {
		http.Error(w, "no auth user", http.StatusUnauthorized)
		log.Println("receiver: no auth user.", err)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		log.Println("receiver: no body.", err)
		return
	}

	if len(body) == 0 {
		http.Error(w, "body is empty", http.StatusBadRequest)
		log.Println("receiver: body is empty")
		return
	}

	cli, err := findClient(ua.Token)
	if err != nil {
		cli = &client{ua: ua}
	}

	text := html.EscapeString(string(body))
	log.Println("message from", ua.Name, ":", text)

	m := &message{cli, nil, text, ""}
	broadcastChan <- m
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "version: %s\ndate: %s\n", version, date)
}

func createFileServer() http.HandlerFunc {
	generatePages()
	dir := http.Dir(cfg.WorkDir)
	fileserver := http.FileServer(dir)

	f := func(w http.ResponseWriter, r *http.Request) {

		if r.URL.Path != "/login.html" {
			token, err := getToken(r)
			if err == http.ErrNoCookie {
				http.Redirect(w, r, "/login.html", http.StatusFound)
				log.Println("redirect to /login.html")
				return
			}

			_, err = auth.GetAuthUser(token)
			if err != nil {
				http.Redirect(w, r, "/login.html", http.StatusFound)
				log.Println("redirect unknown user to /login.html")
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
		log.Println("upload: no token.", err)
		return
	}

	ua, err := auth.GetAuthUser(token)
	if err != nil {
		http.Error(w, "no auth user", http.StatusUnauthorized)
		log.Println("upload: no auth user.", err)
		return
	}

	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, "cannot parse content-type", http.StatusBadRequest)
		log.Println("upload: invalid content-type.", err)
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
			log.Println("upload: cannot get part.", err)
			return
		}
		defer part.Close()

		mediaType, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			http.Error(w, "cannot parse part media type", http.StatusBadRequest)
			log.Println("upload: cannot parse part media type", mediaType, params, err)
			return
		}

		fname := time.Now().Format("20060102150405-") + part.FileName()
		f, err := os.Create(cfg.WorkDir + fname)
		if err != nil {
			http.Error(w, "cannot create file", http.StatusBadRequest)
			log.Println("upload: cannot create file.", fname, err)
			return
		}
		defer f.Close()

		var wr io.Writer
		wr = f

		if filepath.Ext(fname) == ".patch" && cfg.PatchDir != "" {
			patchFile := cfg.PatchDir + fname
			pf, err1 := os.Create(patchFile)
			if err1 != nil {
				http.Error(w, "cannot create patch file", http.StatusBadRequest)
				log.Println("upload: cannot create patch file.", err1)
				return
			}
			wr = io.MultiWriter(f, pf)
			defer pf.Close()
			log.Println("saving patch to", patchFile)
		}

		written, err := io.Copy(wr, part)
		if err != nil {
			http.Error(w, "cannot copy file", http.StatusBadRequest)
			log.Println("upload: cannot copy file.", err)
			return
		}

		fmt.Fprintf(w, "%d bytes sent\n", written)

		text := fmt.Sprintf("file: <a target=\"chaturls\" href=\"%s\">%s</a>", fname, part.FileName())
		log.Println("upload: file from", ua.Name, fname)
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
	http.HandleFunc("/ver", versionHandler)

	go workerRoutine()

	err = http.ListenAndServeTLS(":8085", cfg.WorkDir+"server.pem", cfg.WorkDir+"server.key", nil)
	log.Fatal(err)
}
