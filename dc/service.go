package dc

import (
	"fmt"
	"github.com/golang/glog"
	"google.golang.org/grpc"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

func stackDump() {
	sigChan := make(chan os.Signal)
	go func() {
		stacktrace := make([]byte, 8192*2)
		for _ = range sigChan {
			glog.Error("DUMPING STACK")
			length := runtime.Stack(stacktrace, true)
			glog.Error(string(stacktrace[:length]))
		}
	}()
	signal.Notify(sigChan, syscall.SIGQUIT)
}

/*
 * binds everything together into single service
 */

type DistCron struct {
	node          Node
	dispatcher    Dispatcher
	runner        Runner
	db            *DB
	api           *apiService
	leaderChannel chan bool
	stopChannel   chan bool
}

type rpcInfo struct {
	Addr string    `json:"addr"`
	Ts   time.Time `json:"ts"`
}

func NewDistCron(clusterConfig *ClusterConfig,
	dbHosts []string,
	rpcAddr string,
) (*DistCron, error) {

	db, err := ConnectDB(dbHosts)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	cron := &DistCron{
		stopChannel:   make(chan bool, cChanBuffer),
		leaderChannel: make(chan bool, cChanBuffer),
		db:            db,
		runner:        NewRunner(),
	}

	telemetryChan := make(chan *TelemetryInfo, cChanBuffer)
	if cron.node, err = NewClusterNode(clusterConfig, cron.db, cron.leaderChannel, telemetryChan); err != nil {
		glog.Error(err)
		return nil, err
	}
	cron.dispatcher = NewDispatcher(cron.node, cron, telemetryChan)

	if cron.api, err = NewApiServer(cron.runner, cron.dispatcher, cron); err != nil {
		glog.Error(err)
		return nil, err
	}

	cron.node.Start()
	cron.dispatcher.Start()
	cron.api.Start(rpcAddr)

	go cron.run()

	return cron, nil
}

func (cron *DistCron) Stop() {
	cron.api.Stop()
	cron.dispatcher.Stop()
	cron.node.Stop()
	cron.stopChannel <- true
	glog.Infof("[%s] Service Stop", cron.GetNodeName())
}

func (cron *DistCron) JoinTo(addr []string) (err error) {
	if err = cron.node.Join(addr); err != nil {
		glog.Error(err)
	}
	return
}
func (cron *DistCron) GetLeaderNode() (string, error) {
	node, _, err := cron.node.GetLeader()
	return node, err
}

func (cron *DistCron) IsLeader() bool {
	return cron.node.IsLeader()
}

func (cron *DistCron) GetNodeName() string {
	return cron.node.GetName()
}

func (cron *DistCron) SetRpcForNode(node, addr string) (err error) {
	err = cron.db.Put(fmt.Sprintf("%s/%s", CRPCPrefix, node),
		&rpcInfo{
			Addr: addr,
			Ts:   time.Now(),
		})
	if err != nil {
		glog.Errorf("Can't publish RPC address for a node : %v", err)
	}
	return
}

func (cron *DistCron) GetRpcForNode(node string) (DistCronClient, error) {
	info := rpcInfo{}
	if err := cron.db.Get(fmt.Sprintf("%s/%s", CRPCPrefix, node), &info); err != nil {
		glog.Error(err)
		return nil, err
	}
	conn, err := grpc.Dial(info.Addr, grpc.WithInsecure())
	if err != nil {
		glog.Errorf("fail to dial %s: %v", info.Addr, err)
		return nil, err
	}

	return NewDistCronClient(conn), nil
}

func (cron *DistCron) run() {
	for {
		select {
		case <-cron.stopChannel:
			return
		case leader := <-cron.leaderChannel:
			glog.V(cLogDebug).Infof("[%s] Leader %v", cron.GetNodeName(), leader)
			// we may wish to pause other services
			// but only side effect right now is that node stops collecting telemetry
			// which it automatically does when leadership is lost
		}
	}
}
