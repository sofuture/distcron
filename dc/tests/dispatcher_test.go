package dc

import (
	"distcron/dc"
	"distcron/dc/mocks"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"testing"
)

func mkFailingClient() *mocks.DistCronClient {
	client := new(mocks.DistCronClient)
	client.On("RunJobOnThisNode", context.Background(), (*dc.Job)(nil)).Return(nil, dc.EInternalError)
	return client
}

func mkTelemetryOverload() chan *dc.TelemetryInfo {
	tchan := make(chan *dc.TelemetryInfo)

	go func() {
		/*
			tchan <- &dc.TelemetryInfo{
				Node: "A",
			}
		*/
	}()

	return tchan
}

func mkSerfMembers(names []string) []serf.Member {
	members := make([]serf.Member, len(names))
	for i, _ := range members {
		members[i].Name = names[i]
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

func TestDispatcherNodeTelemetry(t *testing.T) {
	telemetryChannel := mkTelemetryOverload()
	failingClient := mkFailingClient()

	rpc := new(mocks.RpcInfo)
	rpc.On("GetRpcForNode", "A").Return(failingClient, nil)
	rpc.On("GetRpcForNode", "B").Return(failingClient, nil)

	serfMembers := mkSerfMembers([]string{"A", "B"})
	node := mkNode("A", true, serfMembers)
	disp := dc.NewDispatcher(node, rpc, telemetryChannel)
	disp.Start()

	_, err := disp.NewJob(&dc.Job{ContainerName: "hello-world", CpuLimit: 1.0, MemLimitMb: 500})
	require.EqualError(t, err, dc.ENoNodesAvailable.Error())
}
