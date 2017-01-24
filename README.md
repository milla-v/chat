Chat server project
===================

Quick help
----------
	Get: go get github.com/milla-v/chat/...
	Doc: godoc -http=:6060 ; firefox http://localhost:6060/pkg/chat/

Goals and features
------------------

- Extremely simple deployment of private chat server.
- Secure over https.
- Cross platform.
- Web client, console client, API for any client.
- Instant server upgrade using ocean tool.

Get and build chatd, chatc, ocean
---------------------------------

	go get github.com/milla-v/chat/...

Read documentation
------------------

Run document server

    godoc -http=:6060

Open documentation

	firefox http://localhost:6060/pkg/chat/

Run server
------------

    chatd

Open web client

	firefox https://localhost:8085

Deploy to the cloud
-------------------

	ocean -def

Run console client
------------------

	chatc

Useful aliases
--------------

	alias gg='go install github.com/milla-v/chat/...'
	alias nn='terminal-notifier -remove chatc'
	alias gf='git format-patch origin/master'

	gg -- install chat
	nn -- hide chat notification on OSX
	gf -- format patch
