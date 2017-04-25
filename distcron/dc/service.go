package dc

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

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
}

type rpcInfo struct {
	Addr string    `json:"addr"`
	Ts   time.Time `json:"ts"`
}

func NewDistCron(ctx context.Context, clusterConfig *ClusterConfig,
	dbHosts []string,
	rpcAddr string,
) (*DistCron, error) {

	db, err := ConnectDB(dbHosts)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	cron := &DistCron{
		leaderChannel: make(chan bool, cChanBuffer),
		db:            db,
		runner:        NewRunner(),
	}

	telemetryChan := make(chan *TelemetryInfo, cChanBuffer)
	if cron.node, err = NewClusterNode(clusterConfig, cron.db, cron.leaderChannel, telemetryChan); err != nil {
		glog.Error(err)
		return nil, err
	}
	cron.dispatcher = NewDispatcher(cron.node, cron, telemetryChan, nil)

	if cron.api, err = NewApiServer(cron.runner, cron.dispatcher, cron); err != nil {
		glog.Error(err)
		return nil, err
	}

	cron.node.Start(ctx)
	cron.dispatcher.Start(ctx)
	cron.api.Start(ctx, rpcAddr)
	go cron.run(ctx)

	return cron, nil
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

func (cron *DistCron) GetRpcForNode(ctx context.Context, node string) (DistCronClient, error) {
	info := rpcInfo{}
	if err := cron.db.Get(fmt.Sprintf("%s/%s", CRPCPrefix, node), &info); err != nil {
		glog.Error(err)
		return nil, err
	}
	conn, err := grpc.DialContext(ctx, info.Addr, grpc.WithInsecure())
	if err != nil {
		glog.Errorf("fail to dial %s: %v", info.Addr, err)
		return nil, err
	}

	return NewDistCronClient(conn), nil
}

func (cron *DistCron) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case leader := <-cron.leaderChannel:
			glog.V(cLogDebug).Infof("[%s] Leader %v", cron.GetNodeName(), leader)
			// we may wish to pause other services
			// but only side effect right now is that node stops collecting telemetry
			// which it automatically does when leadership is lost
		}
	}
}
