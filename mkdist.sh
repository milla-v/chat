#!/bin/sh

# build for freebsd and create archive for chat server

VERSION=$(git describe --long --tags)
DATE=$(date +%Y-%m-%dT%H:%M:%S%z)

# build for cloud server
GOOS=freebsd GOARCH=amd64 go build -ldflags "-X main.version=$VERSION -X main.date=$DATE" -o chatd-freebsd github.com/milla-v/chat/cmd/chatd

# create destination directory structure
rm -rf dist~
mkdir -p dist~/root/usr/local/www/wet/
mkdir -p dist~/root/usr/local/etc/rc.d/
mkdir -p dist~/root/usr/local/lib/

# copy files
cp chatd-freebsd dist~/root/usr/local/www/wet/chatd
cp chatd dist~/root/usr/local/etc/rc.d/

# copy keys first time only
#cp server.key dist~/root/usr/local/www/wet/
#cp server.pem dist~/root/usr/local/www/wet/

# prepare init script
echo service chatd restart > dist~/root/usr/local/lib/chat.tar.gz-configure.sh
echo echo == done == >> dist~/root/usr/local/lib/chat.tar.gz-configure.sh
chmod +x dist~/root/usr/local/lib/chat.tar.gz-configure.sh

# pack it
cd dist~/root
tar -czf ../../chat.tar.gz *
