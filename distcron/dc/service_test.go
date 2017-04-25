package dc

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"io"
	"testing"
	"time"
)

func testApiCluster(t *testing.T) {

}

func mkApiClient(dialTo string) (DistCronClient, error) {
	if conn, err := grpc.Dial(dialTo, grpc.WithInsecure()); err != nil {
		return nil, err
	} else {
		return NewDistCronClient(conn), nil
	}
}

func mkServiceCluster(t *testing.T) (svcA, svcB *DistCron, err error) {
	//dbHosts := []string{"172.17.8.101:2379"}
	dbHosts := []string{"consul:8500"}

	svcA, err = NewDistCron(&ClusterConfig{
		NodeName: "A",
		BindAddr: "127.0.0.1",
		BindPort: 5006,
	}, dbHosts, ":5556")
	assert.NoError(t, err)

	svcB, err = NewDistCron(&ClusterConfig{
		NodeName: "B",
		BindAddr: "127.0.0.1",
		BindPort: 5007,
	}, dbHosts, ":5557")
	assert.NoError(t, err)

	assert.NoError(t, svcB.node.Join([]string{"127.0.0.1:5006"}))

	for !svcA.IsLeader() && !svcB.IsLeader() {
		time.Sleep(time.Second * 1)
	}

	return
}

func TestBasicService(t *testing.T) {
	svcA, svcB, err := mkServiceCluster(t)
	require.NoError(t, err)
	defer svcA.Stop()
	defer svcB.Stop()

	ctx := context.Background()

	clientA, err := svcA.GetRpcForNode(ctx, "A")
	require.NoError(t, err)
	clientB, err := svcA.GetRpcForNode(ctx, "A")
	require.NoError(t, err)

	testApiBasics(ctx, clientA, t)
	testApiBasics(ctx, clientB, t)
}

func testNodeRoleSwitch(ctx context.Context, svcA, svcB *DistCron, t *testing.T) {
	leader, _ := svcA.GetLeaderNode()
	var other *DistCron
	var otherClient DistCronClient

	clientA, err := svcA.GetRpcForNode(ctx, "A")
	assert.NoError(t, err)
	clientB, err := svcA.GetRpcForNode(ctx, "B")
	assert.NoError(t, err)

	if leader == "A" {
		svcA.Stop()
		other, otherClient = svcB, clientB
	} else {
		svcB.Stop()
		other, otherClient = svcA, clientA
	}

	_, err = otherClient.RunJob(ctx, &Job{
		ContainerName: "hello-world",
		CpuLimit:      1,
		MemLimitMb:    500,
	})
	if other.IsLeader() {
		assert.NoError(t, err)
	} else {
		assert.Error(t, err)
	}

	for other.IsLeader() == false {
		time.Sleep(time.Second * 3)
	}
}

func testApiBasics(ctx context.Context, client DistCronClient, t *testing.T) {
	err := NoNodesAvailableError
	var handle *JobHandle
	for ; err == NoNodesAvailableError; handle, err = client.RunJob(ctx, &Job{
		ContainerName: "hello-world",
		CpuLimit:      1,
		MemLimitMb:    500,
	}) {
		time.Sleep(time.Second * 2)
	}
	require.NoError(t, err)

	_, err = client.StopJob(ctx, handle)
	assert.NoError(t, err)

	_, err = client.GetJobStatus(ctx, handle)
	assert.NoError(t, err)

	output, err := client.GetJobOutput(ctx, handle)
	require.NoError(t, err)
	for {
		data, err := output.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Error(err)
			break
		}
		t.Log(string(data.Data))
	}
}
