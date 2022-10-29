// Create WebSocket connection.
const socket = new WebSocket('ws://localhost:8888/?token=abcd');

// Connection opened
socket.addEventListener('open', function (event) {
    //认证
    a ={type:0,payload:"no123456",channel:"channel1"}
    console.log(JSON.stringify(a))
    socket.send(JSON.stringify(a));
});

// Listen for messages
socket.addEventListener('message', function (event) {
    const e = document.getElementById("messages");
    e.innerHTML = e.innerHTML + 'Message from server '+ event.data +"<br/>";

    console.log('Message from server ', event.data)
});