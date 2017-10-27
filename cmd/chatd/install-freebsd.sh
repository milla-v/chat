set -x
set -e

VER=`git describe --tags --long`
go build -ldflags "-X main.version=$VER"
sudo service chatd stop
sudo cp chatd /usr/local/www/wet/chatd
grep chat_enable /etc/rc.conf || echo chat_enable=\"YES\" >> /etc/rc.conf
sudo service chatd start
sleep 3
tail -20 /var/log/chatd.log
/usr/local/www/wet/chatd -v

