package service
const (
indexHTML = `<html>
<head>
<title>Chat</title>
<meta name="viewport" content="width=device-width">
<style>
.sendbtn { width:80px; height: 60px; }
#textbox { width:100%; height: 60px; font-size: 16px;  }
#sendfilepad { width:50%; height: 60px; font-size: 16px;  }
#msglog { height: 80%; overflow-y: auto; top: 0px; }
body  {
	font-size: large;
	margin-left: 8%;
	margin-right: 8%;
	text-align: justify;
}
p {text-indent: 3%; }
p.noindent { text-indent: 0%; }
.smallcaps { font-variant: small-caps; }
.ts { color: gray; font-size: small; }
pre { margin-left: 6%; background-color: #EEEEEE; padding: 4px 4px 4px 4px; }
</style>
<script>

var showNotification = true;

function notify(s)
{
	if (!showNotification)
		return;

	if (!("Notification" in window)) {
		alert("This browser does not support desktop notification");
		return;
	}

	if (Notification.permission === "granted") {
		var notification = new Notification(s);
		return;
	}

	if (Notification.permission !== 'denied') {
		Notification.requestPermission(function (permission) {
			if (permission === "granted") {
					var notification = new Notification(s);
      			}
    		});
  	}
}

var ws;
var count = 0;

function ws_onclose(e)
{
	setTitle('Chat [off]');
	count = 10;
	window.setTimeout(reconnect, 1000);
}

function setTitle(t) {
	document.title = t;
}

function restoreTitle() {
	setTitle('Chat');
}

function ws_onmessage(e)
{
	console.log(e.data);
	e = JSON.parse(e.data);

	if (e.ping != null && e.ping.ping > 0) {
		e.ping.pong = e.ping.ping;
		ws.send(JSON.stringify(e));
	} else if (e.roster != null){
		roster.innerHTML = e.roster.html;
	} else if (e.message != null){
		if (e.message.notification.length > 0) {
			notify(e.message.notification);
			setTitle('Chat [NEW]');
			window.onfocus = restoreTitle;
		}
		msglog.innerHTML += e.message.html;
	}

	msglog.scrollTop = msglog.scrollHeight;
}

function ws_onopen()
{
	setTitle('Chat');
	msglog.innerHTML += '<p>(type /help then ENTER)</p>\n';
	msglog.scrollTop = msglog.scrollHeight;
}

function reconnect()
{
	setTitle('Chat [off]');

	if (count > 0) {
		window.setTimeout(reconnect, 1000);
		count--;
	} else {
		connect();
	}
}

function connect()
{
	msglog.innerHTML = '';

	ws = new WebSocket("wss://localhost:8085/ws");
	ws.onclose = ws_onclose;
	ws.onmessage = ws_onmessage;
	ws.onopen = ws_onopen;
}

function sendText()
{
	var t = textbox.value;

	if (t.length == 0) {
		return;
	}

	if (t == 'n') {
		showNotification = !showNotification;
		msglog.innerHTML += '<p>(show notifications: ' + showNotification + ')</p>\n';
		textbox.value = '';
		msglog.scrollTop = msglog.scrollHeight;
		return;
	} else if (t == 'f') {
		toggleSendFile();
		textbox.value = '';
		return;
	}

	m = { message: { text: t }};
	ws.send(JSON.stringify(m));
	textbox.value = '';
}

function toggleSendFile()
{
	sendfilepad.style.display = (sendfilepad.style.display == '') ? 'none' : '';
}

function msgclick(o)
{
	m = { action: { id: id, click: 1 }};
	ws.send(JSON.stringify(m));
}

function keypress(event)
{
	if (event.target === textbox && event.keyCode === 13) {
		sendText();
		event.returnValue = false;
	}
}

function sendFile() {
	if (window.uploadfile.files.length == 0) {
		return;
	}

	var req = new XMLHttpRequest();
	var formData = new FormData();
	formData.append('uploadfile', window.uploadfile.files[0]);

	req.open('POST', 'https://localhost:8085/upload', true);
	req.onload = function(e) {
		console.log(this.response);
	};
	req.onerror = function() {
		msglog.innerHTML += '<div>Cannot send file</div>';
	}
	req.send(formData);
}

</script>
</head>
<body onload="connect()">
<div id="msglog"></div>
<br>
<textarea id="textbox" onkeypress="keypress(event)"></textarea>
<table style="display: none; border: solid 1px pink" id="sendfilepad">
	<tr>
		<td>
			<input type="file" id="uploadfile" name="uploadfile">
		</td>
		<td >
			<button class="sendbtn" onclick="sendFile()">Send file</button>
		</td>
	</tr>
</table>

<div>
	<a target="chaturls" href="/history.html">history</a>
	<a href="/login.html">relogin</a>
	<button onclick="toggleSendFile()">File...</button>
	<span id="roster">nobody in the room</span>
</div>
</body>
</html>
`

loginHTML = `<html>
<head>
	<title>Login to chat</title>
	<meta name="viewport" content="width=device-width">
</head>
<body>

	<h3>Log into chat</h3>
	<form method="POST" action="/auth">
		username:<br>
		<input name="user"/><br><br>
		password:<br>
		<input type="password" name="password"/><br>
		<br>
		<button type="submit">Login</button>
		<input type="hidden" name="redirect" value="1"/><br>
	</form>
	<br><br>

	<h3>Register</h3>
	<form method="POST" action="/register">
		username:<br>
		<input name="user"/><br><br>
		email:<br>
		<input type="email" name="email"/><br>
		<br>
		<button type="submit">Register</button>
	</form>

</body>
</html>
`

)
