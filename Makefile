all:
	PATH=${GOPATH}/bin:${PATH} protoc -I dc/ dc/dcron.proto --go_out=plugins=grpc:dc
