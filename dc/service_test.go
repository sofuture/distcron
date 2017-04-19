package dc

import (
	"github.com/stretchr/testify/assert"
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
	dbHosts := []string{"127.0.0.1:8500"}

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
	assert.NoError(t, err)
	defer svcA.Stop()
	defer svcB.Stop()

	clientA, err := svcA.GetRpcForNode("A")
	assert.NoError(t, err)
	clientB, err := svcA.GetRpcForNode("A")
	assert.NoError(t, err)

	testApiBasics(clientA, t)
	testApiBasics(clientB, t)
}

func testNodeRoleSwitch(svcA, svcB *DistCron, t *testing.T) {
	leader, _ := svcA.GetLeaderNode()
	var other *DistCron
	var otherClient DistCronClient

	clientA, err := svcA.GetRpcForNode("A")
	assert.NoError(t, err)
	clientB, err := svcA.GetRpcForNode("B")
	assert.NoError(t, err)

	if leader == "A" {
		svcA.Stop()
		other, otherClient = svcB, clientB
	} else {
		svcB.Stop()
		other, otherClient = svcA, clientA
	}

	_, err = otherClient.RunJob(context.Background(), &Job{
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

func testApiBasics(client DistCronClient, t *testing.T) {
	handle, err := client.RunJob(context.Background(), &Job{
		ContainerName: "hello-world",
		CpuLimit:      1,
		MemLimitMb:    500,
	})
	assert.NoError(t, err)

	_, err = client.StopJob(context.Background(), handle)
	assert.NoError(t, err)

	_, err = client.GetJobStatus(context.Background(), handle)
	assert.NoError(t, err)

	output, err := client.GetJobOutput(context.Background(), handle)
	assert.NoError(t, err)
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
