Chat server project
===================

Quick help
----------
	Get: go get github.com/milla-v/chat/...
	Doc: godoc -http=:6060 ; open http://localhost:6060/pkg/chat/

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

Run doc server

    godoc -http=:6060

Open doc

	open http://localhost:6060/pkg/chat/

Run server
------------

    chatd

Open web client

	open https://localhost:8085

Deploy to the cloud
-------------------

	ocean -def

Run console client
------------------

	chatc
 
