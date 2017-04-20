package dc

import (
	"math"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/net/context"
)

/*
 * Dispatcher receives job run requests and decides which node should run it
 *
 * Also listens to cluster membership and telemetry events from Node
 * to be aware of current workloads
 *
 * If there are no nodes matching the load available, it simply rejects new job
 *
 */

type dispatcher struct {
	sync.Mutex
	api              RpcInfo
	node             Node
	nodeInfo         map[string]*nodeInfo
	telemetryChannel chan *TelemetryInfo
	cancel           context.CancelFunc
}

type nodeInfo struct {
	errorCount   int
	backoffUntil time.Time
	memAvailable int64
	cpuAvailable float64
}

const cMB = 2 << 19
const cMinRAM = cMB * 500
const cMinCPU = 0.0

func NewDispatcher(node Node, api RpcInfo, telemetryChannel chan *TelemetryInfo) Dispatcher {
	return &dispatcher{
		api:              api,
		node:             node,
		nodeInfo:         make(map[string]*nodeInfo),
		telemetryChannel: telemetryChannel,
	}
}

func (d *dispatcher) Start() {
	var ctx context.Context
	ctx, d.cancel = context.WithCancel(context.Background())
	go d.run(ctx)
}

func (d *dispatcher) Stop() {
	d.cancel()
}

func (d *dispatcher) NewJob(ctx context.Context, job *Job) (*JobHandle, error) {
	for retry := 0; retry <= d.node.SerfMembersCount(); retry++ {
		select {
		case <-ctx.Done():
			glog.Errorf("[%s] Job %v rejected, context %v", d.node.GetName(), job, ctx.Err())
			return nil, ERequestTimeout
		default:
		}

		node, err := d.getAvailableNode(job)
		if err != nil {
			glog.Errorf("[%s] Job %v, retry=%d - no more nodes available, giving up", d.node.GetName(), job, retry)
			return nil, ENoNodesAvailable
		}

		if handle, err := d.newJob(ctx, job, node); err == nil {
			return handle, err
		}
	}

	glog.Errorf("[%s] Job %v, max retries, giving up", d.node.GetName(), job)
	return nil, ENoNodesAvailable
}

func (d *dispatcher) newJob(ctx context.Context, job *Job, node string) (*JobHandle, error) {
	rpc, err := d.api.GetRpcForNode(ctx, node)
	if err != nil {
		glog.Error(err)
		// maybe this node just joined and did not register yet - backoff
		d.nodeFailed(node)
		return nil, err
	}

	handle, err := rpc.RunJobOnThisNode(ctx, job)
	if err != nil {
		glog.Errorf("[%s] Job %v failed to run on node %s : %v", d.node.GetName(), job, node, err)
		d.nodeFailed(node)
		return nil, err
	}

	glog.V(cLogDebug).Infof("[%s] Job %v running on %s, handle %v", d.node.GetName(), job, node, handle)
	d.clearNodeErrors(node)
	return handle, nil
}

func fitForJob(info *nodeInfo, job *Job) bool {
	return (int64(info.memAvailable)-job.MemLimitMb*cMB >= cMinRAM) ||
		(info.cpuAvailable-float64(job.CpuLimit) >= cMinCPU)
}

func (d *dispatcher) getAvailableNode(job *Job) (string, error) {
	members := d.node.SerfMembers()
	now := time.Now()

	d.Lock()
	defer d.Unlock()

	for _, member := range members {
		if member.Status == serf.StatusAlive {
			if info := d.getNodeInfo(member.Name); info.backoffUntil.Before(now) && fitForJob(info, job) {
				glog.V(cLogDebug).Infof("[%s] selected node %s:%+v to run %v",
					d.node.GetName(), member.Name, info, job)
				return member.Name, nil
			} else {
				glog.V(cLogDebug).Infof("[%s] job %v can't run on %s:%+v",
					d.node.GetName(), job, member.Name, info)
			}
		}
	}

	glog.V(cLogDebug).Infof("[%s] Couldn't find any nodes for job %v", d.node.GetName(), job)
	return "", ENoNodesAvailable
}

/*
 * main dispatcher activity is monitoring other nodes load, while in leadership mode
 *
 */
func (d *dispatcher) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case tm := <-d.telemetryChannel:
			d.updateTelemetry(tm)
		}
	}
}

func (d *dispatcher) updateTelemetry(tm *TelemetryInfo) {
	d.Lock()
	defer d.Unlock()

	info := d.getNodeInfo(tm.Node)
	cpu := 0.0
	for _, proc := range tm.Cpu {
		cpu += float64(proc.Cores)
	}
	info.cpuAvailable = cpu - tm.Load.Load5
	info.memAvailable = int64(tm.Mem.Available)

	glog.V(cLogDebug).Infof("[%s] Telemetry in : %v => %+v", d.node.GetName(), tm, info)
}

func (d *dispatcher) getNodeInfo(name string) *nodeInfo {
	info, there := d.nodeInfo[name]
	if !there {
		info = &nodeInfo{}
		d.nodeInfo[name] = info
	}
	return info
}

func (d *dispatcher) nodeFailed(name string) {
	d.Lock()
	defer d.Unlock()

	info := d.nodeInfo[name]
	info.errorCount++
	info.backoffUntil = time.Now().Add(backoff(info.errorCount))
}

func (d *dispatcher) clearNodeErrors(name string) {
	d.Lock()
	defer d.Unlock()

	info := d.nodeInfo[name]
	info.errorCount = 0
}

const cMaxDelay = time.Minute * 5

func backoff(errCount int) time.Duration {
	delay := time.Millisecond * time.Duration(int64(1000*(math.Pow(2, float64(errCount))-1)/2))
	if delay > cMaxDelay {
		return cMaxDelay
	} else {
		return delay
	}
}
