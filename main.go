package main

import (
	"distcron/dc"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"os"
	"os/signal"
)

var nodeName = flag.String("node", "", "Node name, should be unique")
var nodeBindPort = flag.Int("nodeBindPort", 5000, "Node cluster RPC port")
var nodeBindAddr = flag.String("nodeBindAddr", "127.0.0.1", "Node clust RPC address")
var apiPort = flag.Int("apiPort", 5050, "GRPC API port")
var etcd = flag.String("etcd", "172.17.8.101:2379", "ETCD address and port")
var joinTo = flag.String("join", "", "Other cluster addresses to join to")

func main() {
	flag.Parse()

	if *nodeName == "" {
		*nodeName = fmt.Sprintf("dc_%s:%d", *nodeBindAddr, *nodeBindPort)
	}

	svc, err := dc.NewDistCron(&dc.ClusterConfig{
		NodeName: *nodeName,
		BindAddr: *nodeBindAddr,
		BindPort: *nodeBindPort,
	}, []string{*etcd}, fmt.Sprintf(":%d", *apiPort))
	if err != nil {
		glog.Fatal(err)
	}

	if *joinTo != "" {
		svc.JoinTo([]string{*joinTo})
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	<-c
	svc.Stop()
}
