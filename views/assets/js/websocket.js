
function WSClient(url, onmessage) {
    if(window.WebSocket != undefined) {    // 检测是否支持websocket
        var connection = new WebSocket(url);
        // readyState为open时触发
        connection.onopen = function wsOpen(event) {
            console.log("Connected to localhost:8080");
        };
        // readyState为close时触发
        connection.onclose = function wsClose() {
            console.log("WebSocket is closed")
        };
        // 客户端收到服务端信息触发
        connection.onmessage = onmessage
    }
}