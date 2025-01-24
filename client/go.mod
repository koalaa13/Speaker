module client

go 1.23

require (
	github.com/google/uuid v1.6.0
	github.com/gordonklaus/portaudio v0.0.0-20230709114228-aafa478834f5
	github.com/gotk3/gotk3 v0.6.1
	google.golang.org/grpc v1.69.4
	proto v0.0.0
	types v0.0.0
)

require (
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241015192408-796eee8c2d53 // indirect
	google.golang.org/protobuf v1.36.2 // indirect
)

replace proto => ../proto

replace types => ../types
