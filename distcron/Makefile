all: proto tests build

tests:
	go test -v distcron/dc/... -alsologtostderr

build:
	go build -o /code/build/distcron distcron 

proto:
	protoc -I dc/ dc/dcron.proto --go_out=plugins=grpc:dc

.PHONY: build tests proto all
