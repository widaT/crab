# crab

a simple websocket server base on [poller](https://github.com/widaT/poller).

inspired by [1m-go-websockets](https://github.com/eranyanay/1m-go-websockets)



## usage

run

```bash
$ run cmd/server/server.go
#open a new terminal 
$ run cmd/client/client.go
#open a new terminal
$ curl -X POST -d "sn=no123456&msg=88888888888" http://localhost:9333/send_msg
```