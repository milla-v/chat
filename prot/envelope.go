// Package prot provides protocol structures for communication between chat server and client.
package prot

import (
	"time"
)

// Ping is a client ping message
type Ping struct {
	Timestamp time.Time `json:"ts"`   // ping timestamp
	Ping      int       `json:"ping"` // server should set incremental ping number
	Pong      int       `json:"pong"` // client should respond with the same number
}

// Message is a conversation message
type Message struct {
	Ts            time.Time `json:"ts"`             // timestamp
	Name          string    `json:"name"`           // username
	Text          string    `json:"text"`           // plain text for console clients
	HTML          string    `json:"html"`           // html text for browsers
	Notification  string    `json:"notification"`   // plain notification for browsers
	Color         string    `json:"color"`          // RGB color
	ColorXterm256 string    `json:"color_xterm256"` // xterm color number suitable for \033[%sm formatting
}

// Roster is a list of online users
type Roster struct {
	Ts   time.Time `json:"ts"`   // timestamp
	HTML string    `json:"html"` // html text for browsers
}

// Envelope is a top level communication structure. Includes all another submessages.
type Envelope struct {
	Message *Message `json:"message,omitempty"` // conversation message
	Ping    *Ping    `json:"ping,omitempty"`    // ping message
	Roster  *Roster  `json:"roster,omitempty"`  // roster (list of users) message
}
