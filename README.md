# crab

golang写的websocket服务，支持超过连接数（实测12w+稳定连接，程序自带压测工具，感兴趣可以测试下）。

受[1m-go-websockets](https://github.com/eranyanay/1m-go-websockets)启发

## usage

run

```bash
$ run cmd/server/server.go
#open a new terminal 
$ run cmd/client/client.go
#open a new terminal
$ curl -X POST -d "sn=no123456&msg=msg1" http://localhost:9333/send_msg
$ curl -X POST -d "channel=channel1&msg=broadcastinchannel" http://localhost:9333/broadcastinchannel
$ curl -X POST -d "msg=broadcastmsg" http://localhost:9333/broadcast
```