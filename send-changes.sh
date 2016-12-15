#!/bin/bash

# send file to the chat

HOST=${HOST:-wet.voilokov.com}
COOKIE_FNAME=cookies~.txt

if [ ! -f ${COOKIE_FNAME} ] ; then
	read -s -p password? PASSWORD
	echo
	curl -c ${COOKIE_FNAME} -k -s https://${HOST}:8085/auth -d "user=console&password=${PASSWORD}"
fi

[ -z "$1" ] && { echo "no description" ; exit; }

rm -f 00*-*.patch
git commit -m "$1"
git format-patch origin/master

PATCHES=`ls 00*-*.patch`

[ -z "${PATCHES}" ] && { echo "no changes to send. 'Use git add -A' to mark changes for send."; exit 1; }

for p in ${PATCHES}; do
	echo Sending $p to ${HOST}
	curl -b ${COOKIE_FNAME} -k https://${HOST}:8085/upload -F "filename=@$p"
done
