module example.com/chainflow-example

go 1.22

require (
	chainflow-data-gateway v0.0.0
	github.com/gorilla/websocket v1.5.3
)

replace chainflow-data-gateway => ../..
