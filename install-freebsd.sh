set -x
set -e

git tag 1 | head
VER=`git describe --tags --long`
cd cmd/chatd
go build -ldflags "-X service.Version=$VER"
cd -
sudo service chatd stop | head
sudo cp cmd/chatd/chatd /usr/local/www/wet/chatd
sudo cp chatd /usr/local/etc/rc.d
grep chat_enable /etc/rc.conf || sudo sh -c 'echo chat_enable=\"YES\" >> /etc/rc.conf'
sudo service chatd start
sleep 3
tail -20 /var/log/chatd.log
/usr/local/www/wet/chatd -version
