package dc

import (
	"testing"

	"distcron/dc"
	"distcron/dc/mocks"

	"github.com/hashicorp/serf/serf"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

var testJob *dc.Job = &dc.Job{ContainerName: "hello-world", CpuLimit: 1.0, MemLimitMb: 500}

func mkFailingClient(ctx context.Context) *mocks.DistCronClient {
	client := new(mocks.DistCronClient)
	client.On("RunJobOnThisNode", ctx, testJob).Return(nil, dc.InternalError)
	return client
}

func mkClient(ctx context.Context, handle string) *mocks.DistCronClient {
	client := new(mocks.DistCronClient)
	client.On("RunJobOnThisNode", ctx, testJob).Return(&dc.JobHandle{handle}, nil)
	return client
}

func mkSerfMembers(names []string) []serf.Member {
	members := make([]serf.Member, len(names))
	for i, _ := range members {
		members[i].Name = names[i]
		members[i].Status = serf.StatusAlive
	}
	return members
}

func mkNode(name string, leader bool, members []serf.Member) *mocks.Node {
	node := new(mocks.Node)
	node.On("GetName").Return(name)
	node.On("IsLeader").Return(leader)
	node.On("SerfMembers").Return(members)
	node.On("SerfMembersCount").Return(len(members))
	return node
}

func mkTelemetry(name string, availCpu int32, availRam uint64, load5 float64) *dc.TelemetryInfo {
	return &dc.TelemetryInfo{
		Node: name,
		Mem:  &mem.VirtualMemoryStat{Available: availRam},
		Cpu:  []cpu.InfoStat{cpu.InfoStat{Cores: availCpu}},
		Load: &load.AvgStat{Load5: load5},
	}
}

func TestDispatcherBasics(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	telemetryChannel := make(chan *dc.TelemetryInfo)
	telemetryUpdate := make(chan bool, 2)

	rpc := new(mocks.RpcInfo)

	serfMembers := mkSerfMembers([]string{"A", "B"})
	node := mkNode("A", true, serfMembers)

	disp := dc.NewDispatcher(node, rpc, telemetryChannel, telemetryUpdate)
	disp.Start(ctx)

	// nodes are available, but no telemetry ever receieved,
	// will fail as resources are unknown, won't try to invoke RPC
	_, err := disp.NewJob(ctx, testJob)
	require.EqualError(t, err, dc.NoNodesAvailableError.Error())

	// report resource availability, should return from node B
	rpc.On("GetRpcForNode", ctx, "B").Return(mkClient(ctx, "B"), nil).Once()
	telemetryChannel <- mkTelemetry("A", 2, 5000, 2.5)
	telemetryChannel <- mkTelemetry("B", 2, 2000, 0.5)
	<-telemetryUpdate
	<-telemetryUpdate
	handle, err := disp.NewJob(ctx, testJob)
	require.NoError(t, err)
	require.EqualValues(t, &dc.JobHandle{"B"}, handle)

	// node B starts to fail, should fail now - A doesn't have any resources available
	rpc.On("GetRpcForNode", ctx, "B").Return(mkFailingClient(ctx), nil).Once()
	handle, err = disp.NewJob(ctx, testJob)
	require.EqualError(t, err, dc.NoNodesAvailableError.Error())

	// node A gets resources, B is still in back-off state, A should be selected
	telemetryChannel <- mkTelemetry("A", 2, 5000, 0.1)
	<-telemetryUpdate
	rpc.On("GetRpcForNode", ctx, "A").Return(mkClient(ctx, "A"), nil).Once()
	testJob.CpuLimit = 1.3
	handle, err = disp.NewJob(ctx, testJob)
	require.NoError(t, err)
	require.EqualValues(t, &dc.JobHandle{"A"}, handle)
}
