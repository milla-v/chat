package service
const (
index_html = `<html>
<head>
<title>Chat</title>
<meta name="viewport" content="width=device-width">
<style>
.msg { border-radius: 10px 10px 10px 10px; padding: 8px 8px 8px 8px; left: 200px; }
.sendbtn { width:80px; height: 60px; }
#textbox { width:320px; height: 60px; font-size: 16px;  }
#msglog { width:400px; height: 416px; overflow-y: auto; border: 1px solid gray; top: 0px; }
#roster { width:80px; height: 100px; overflow-y: auto; border: 1px solid gray; position: absolute; left: 420px; top: 8px; }
</style>
<script>
function notify(s)
{
	if (!enableNotify.checked)
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
var myname;
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
	if (textbox.value.length == 0) {
		return;
	}
	m = { message: { text: textbox.value }};
	ws.send(JSON.stringify(m));
	textbox.value = '';
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
<div id="roster"></div>
<br>
<table>
	<tr>
		<td style="border: solid 1px pink">
			<textarea id="textbox" onkeypress="keypress(event)"></textarea>
		</td>
		<td>
			<button class="sendbtn" onclick="sendText()">Send text</button>
		</td>
	</tr>
	<tr>
		<td style="border: solid 1px pink">
			<input type="file" id="uploadfile" name="uploadfile">
		</td>
		<td>
			<button class="sendbtn" onclick="sendFile()">Send file</button>
		</td>
	</tr>
</table>

<button onclick="textbox.value='/roster'; sendText();">Help</button>
&nbsp;&nbsp;&nbsp;
<input id="enableNotify" type="checkbox" checked>Enable notifications</input>
<div>
<br>
version: {version}, date: {date}<br>
<a target="chaturls" href="/history.html">history</a>
<a href="/login.html">relogin</a>
</div>
</body>
</html>












`

login_html = `<html>
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
