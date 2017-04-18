package dc

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/docker/leadership"
	"github.com/golang/glog"
	"github.com/hashicorp/serf/serf"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"net"
	"time"
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

type Node struct {
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
	leaderChannel chan bool, telemetryChannel chan *TelemetryInfo) (node *Node, err error) {
	node = &Node{
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

	node.serf, err = serf.Create(serfConfig)
	if err != nil {
		glog.Error(err)
		return nil, err
	}

	return
}

func (node *Node) Start() {
	go node.run()
}

func (node *Node) Stop() {
	node.candidate.Resign()
	node.serf.Shutdown()
}

type TelemetryInfo struct {
	node string
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

func (node *Node) queryRequestTelemetry() {
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
		if resp, ok := <-respChannel; !ok {
			break
		} else {
			tm := TelemetryInfo{}
			if err := gob.NewDecoder(bytes.NewReader(resp.Payload)).Decode(&tm); err != nil {
				glog.Error(err)
			} else {
				tm.node = resp.From
				node.telemetryChannel <- &tm
			}
		}
	}
}

func (node *Node) run() {
	serfShutdownChannel := node.serf.ShutdownCh()

	node.candidate = leadership.NewCandidate(node.db.store, CLeadershipKey,
		node.config.NodeName, cDefaultLeaderLock)

	leaderChannel := make(chan bool, cChanBuffer)
	stopElectionChannel := make(chan bool, cChanBuffer)
	go node.electionLoop(leaderChannel, stopElectionChannel)

	var telemetryTicker *time.Ticker
	var telemetryTickerChannel <-chan time.Time = make(<-chan time.Time)

	for {
		select {
		case <-serfShutdownChannel:
			glog.Infof("[%s] Node shutdown", node.GetName())
			stopElectionChannel <- true
			return
		case leader := <-leaderChannel:
			node.leaderChannel <- leader
			if leader {
				telemetryTicker = time.NewTicker(cTelemetryAcquisitionPeriod)
				telemetryTickerChannel = telemetryTicker.C
			} else if telemetryTicker != nil {
				telemetryTicker.Stop()
			}
			glog.Infof("[%s] Leader status : %v", node.GetName(), leader)

		case <-telemetryTickerChannel:
			glog.Infof("[%s] Requesting telemetry", node.GetName())
			go node.queryRequestTelemetry()
		case serfEvent := <-node.serfChannel:
			glog.V(cLogDebug).Infof("[%s] %v", node.config.NodeName, serfEvent)

			if serfEvent.EventType() == serf.EventQuery {
				query := serfEvent.(*serf.Query)

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
		}
	}
}

func (node *Node) electionLoop(leaderChannel chan bool, stopChannel chan bool) {
	electionChan, errorChan := node.candidate.RunForElection()

	quit := true
	for {
		select {
		case <-stopChannel:
			glog.Infof("[%s] Stop election loop", node.GetName())
			node.candidate.Stop()
			quit = true
		case err := <-errorChan:
			glog.Errorf("[%s] Error from election %v", node.GetName(), err)
			leaderChannel <- false
			if quit {
				return
			}
			time.Sleep(cSleepBetweenElections)
			electionChan, errorChan = node.candidate.RunForElection()
		case elected := <-electionChan:
			leaderChannel <- elected
		}
	}
}

func (node *Node) GetName() string {
	return node.config.NodeName
}

func (node *Node) IsLeader() bool {
	return node.candidate.IsLeader()
}

func (node *Node) GetLeader() (name string, addr net.IP, err error) {
	if kv, err := node.db.store.Get(CLeadershipKey); err != nil {
		glog.Errorf("[%s] : cannot fech leader: %v", node.config.NodeName, err)
		return "", nil, err
	} else {
		name = string(kv.Value)
	}

	for _, n := range node.serf.Members() {
		if n.Name == name {
			addr = n.Addr
			return
		}
	}

	return "", nil, fmt.Errorf("Node %s not found", name)
}
