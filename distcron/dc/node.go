package dc

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"net"
	"time"

	"github.com/docker/leadership"
	"github.com/golang/glog"
	"github.com/hashicorp/serf/serf"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"golang.org/x/net/context"
)

/*
 * 1. Maintains membership within a cluster
 * 2. Continuously runs for leader within a cluster
 * 3. Acquires telemetry from other nodes when in leader mode
 *
 * Nodes within cluster are expected to have unique node names
 * Serf also has snapshotting feature to automatically restore cluster membership after restart
 *
 */
type ClusterConfig struct {
	NodeName string

	BindAddr string
	BindPort int

	GRPCAddress string
}

type queryResponder func(query *serf.Query) (interface{}, error)

type cronNode struct {
	cancel           context.CancelFunc
	config           *ClusterConfig
	serfChannel      chan serf.Event
	serf             *serf.Serf
	candidate        *leadership.Candidate
	leaderChannel    chan bool
	telemetryChannel chan *TelemetryInfo
	db               *DB
	queryResponders  map[string]queryResponder
}

const cElectionWaitTimeout = time.Second * 20
const cSleepBetweenElections = time.Second * 10
const cDefaultLeaderLock = time.Second * 20
const cQueryTimeout = time.Second * 15
const cTelemetryAcquisitionPeriod = time.Second * 15 // must be > then cQueryTimeout
const cQueryTelemetry = "DC_TELE"

func NewClusterNode(config *ClusterConfig, db *DB,
	leaderChannel chan bool, telemetryChannel chan *TelemetryInfo) (Node, error) {
	node := &cronNode{
		db:               db,
		config:           config,
		serfChannel:      make(chan serf.Event, cChanBuffer),
		leaderChannel:    leaderChannel,
		telemetryChannel: telemetryChannel,
	}
	node.queryResponders = map[string]queryResponder{
		cQueryTelemetry: queryRespondTelemetry,
	}

	serfConfig := serf.DefaultConfig()
	serfConfig.QueryResponseSizeLimit = 4096
	serfConfig.EventCh = node.serfChannel
	serfConfig.RejoinAfterLeave = true
	serfConfig.MemberlistConfig.BindAddr = config.BindAddr
	serfConfig.MemberlistConfig.BindPort = config.BindPort
	serfConfig.NodeName = config.NodeName

	var err error
	node.serf, err = serf.Create(serfConfig)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	return node, err
}

func (node *cronNode) Start(parentCtx context.Context) {
	ctx, _ := context.WithCancel(parentCtx)
	go node.run(ctx)
}

type TelemetryInfo struct {
	Node string
	Mem  *mem.VirtualMemoryStat `json:"mem"`
	Cpu  []cpu.InfoStat         `json:"cpu"`
	Load *load.AvgStat          `json:"load"`
}

func queryRespondTelemetry(query *serf.Query) (interface{}, error) {
	var err error
	info := TelemetryInfo{}
	if info.Load, err = load.Avg(); err != nil {
		glog.Error(err)
	}
	if info.Cpu, err = cpu.Info(); err != nil {
		glog.Error(err)
	}
	if info.Mem, err = mem.VirtualMemory(); err != nil {
		glog.Error(err)
	}

	return info, nil
}

func (node *cronNode) queryRequestTelemetry(ctx context.Context) {
	q, err := node.serf.Query(cQueryTelemetry, []byte{}, &serf.QueryParam{
		RequestAck: false,
		Timeout:    cQueryTimeout,
	})
	if err != nil {
		glog.Error(err)
		return
	}
	defer q.Close()

	respChannel := q.ResponseCh()
	for !q.Finished() {
		var resp serf.NodeResponse
		var ok bool

		select {
		case <-ctx.Done():
			return
		case resp, ok = <-respChannel:
			if !ok {
				return
			}
		}

		tm := TelemetryInfo{}
		if err := gob.NewDecoder(bytes.NewReader(resp.Payload)).Decode(&tm); err != nil {
			glog.Error(err)
		} else {
			tm.Node = resp.From
			node.telemetryChannel <- &tm
		}
	}
}

func (node *cronNode) processSerfQuery(query *serf.Query) {
	if fn, there := node.queryResponders[query.Name]; there {
		var encBuffer bytes.Buffer
		gobEncoder := gob.NewEncoder(&encBuffer)

		if response, err := fn(query); err != nil {
			glog.Errorf("[%s] Error responding query %v: %v",
				node.config.NodeName, query, err)
		} else if err = gobEncoder.Encode(response); err != nil {
			glog.Errorf("[%s] Cannot encode response to query %v : %v", response, err)
		} else if err = query.Respond(encBuffer.Bytes()); err != nil {
			glog.Errorf("[%s] Failed to respond to query: %v, answer was %d bytes", node.GetName(), err, len(encBuffer.Bytes()))
		}
	}
}

func (node *cronNode) run(ctx context.Context) {
	leaderChannel := make(chan bool, cChanBuffer)
	go node.electionLoop(ctx, leaderChannel)

	var telemetryTicker *time.Ticker
	var telemetryTickerChannel <-chan time.Time = make(<-chan time.Time)

	for {
		select {
		case <-ctx.Done():
			glog.Infof("[%s] Node shutdown", node.GetName())
			glog.Error(node.serf.Shutdown())
			return
		case leader := <-leaderChannel:
			node.leaderChannel <- leader
			if leader {
				telemetryTicker = time.NewTicker(cTelemetryAcquisitionPeriod)
				telemetryTickerChannel = telemetryTicker.C
				go node.queryRequestTelemetry(ctx)
			} else if telemetryTicker != nil {
				telemetryTicker.Stop()
			}
			glog.Infof("[%s] Leader status : %v", node.GetName(), leader)

		case <-telemetryTickerChannel:
			glog.Infof("[%s] Requesting telemetry", node.GetName())
			// we could create a timeout context but serf query have internal timeout
			// so just to ensure once we're shutting down that would be picked up as well
			go node.queryRequestTelemetry(ctx)
		case serfEvent := <-node.serfChannel:
			glog.V(cLogDebug).Infof("[%s] %v", node.config.NodeName, serfEvent)
			if serfEvent.EventType() == serf.EventQuery {
				go node.processSerfQuery(serfEvent.(*serf.Query))
			}
		}
	}
}

func (node *cronNode) electionLoop(ctx context.Context, leaderChannel chan bool) {
	node.candidate = leadership.NewCandidate(node.db.store, CLeadershipKey,
		node.config.NodeName, cDefaultLeaderLock)
	electionChan, errorChan := node.candidate.RunForElection()

	quit := false
	for {
		select {
		case <-ctx.Done():
			glog.Infof("[%s] Stop election loop", node.GetName())
			node.candidate.Stop()
			quit = true
		case err := <-errorChan:
			glog.Errorf("[%s] Error from election %v", node.GetName(), err)
			leaderChannel <- false
			if quit {
				return
			}
			glog.Infof("[%s] Will retry election in %v", node.GetName(), cSleepBetweenElections)
			time.Sleep(cSleepBetweenElections)
			electionChan, errorChan = node.candidate.RunForElection()
		case elected := <-electionChan:
			leaderChannel <- elected
		}
	}
}

func (node *cronNode) GetName() string {
	return node.config.NodeName
}

func (node *cronNode) IsLeader() bool {
	return node.candidate.IsLeader()
}

func (node *cronNode) GetLeader() (name string, addr net.IP, err error) {
	if kv, err := node.db.store.Get(CLeadershipKey); err != nil {
		glog.Errorf("[%s] : cannot fech leader: %v", node.config.NodeName, err)
		return "", nil, err
	} else {
		name = string(kv.Value)
	}

	for _, n := range node.serf.Members() {
		if n.Name == name {
			return name, n.Addr, err
		}
	}

	return "", nil, fmt.Errorf("Node %s not found", name)
}

func (node *cronNode) SerfMembersCount() int {
	return node.serf.NumNodes()
}

func (node *cronNode) SerfMembers() []serf.Member {
	return node.serf.Members()
}

func (node *cronNode) Join(addr []string) error {
	_, err := node.serf.Join(addr, true)
	return err
}
