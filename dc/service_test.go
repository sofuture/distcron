package dc

import (
	"github.com/golang/glog"
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

func TestService(t *testing.T) {
	dbHosts := []string{"172.17.8.101:2379"}

	svcA, err := NewDistCron(&ClusterConfig{
		NodeName: "A",
		BindAddr: "127.0.0.1",
		BindPort: 5006,
	}, dbHosts, ":5556")
	if err != nil {
		t.Error(err)
		return
	}
	defer svcA.Stop()

	time.Sleep(time.Second * 1)

	svcB, err := NewDistCron(&ClusterConfig{
		NodeName: "B",
		BindAddr: "127.0.0.1",
		BindPort: 5007,
	}, dbHosts, ":5557")
	if err != nil {
		t.Error(err)
		return
	}
	defer svcB.Stop()

	time.Sleep(time.Second * 1)

	if _, err := svcB.node.serf.Join([]string{"127.0.0.1:5006"}, true); err != nil {
		t.Error(err)
		return
	}

	for !svcA.IsLeader() && !svcB.IsLeader() {
		time.Sleep(time.Second * 1)
	}

	clientA, err := svcA.GetRpcForNode("A")
	clientB, err := svcA.GetRpcForNode("B")

	handle, err := clientA.RunJob(context.Background(), &Job{
		ContainerName: "hello-world",
		CpuLimit:      1,
		MemLimitMb:    500,
	})

	if err != nil {
		t.Error(err)
		return
	} else {
		t.Logf("Got job handle %v", handle)
	}
}

// it doesn't work within single process likely due to etcd client
// so need be rewritten to have all that happening in separate processes
func testNodeRoleSwitch(svcA, svcB DistCronClient, handle *JobHandle) {
	leader, _ := svcA.GetLeaderNode()
	var other *DistCron
	var otherClient DistCronClient
	if leader == "A" {
		svcA.Stop()
		other, otherClient = svcB, clientB
	} else {
		svcB.Stop()
		other, otherClient = svcA, clientA
	}

	for other.IsLeader() == false {
		time.Sleep(time.Second * 3)
		if false {
			testApiBasics(otherClient, handle, t)
		}
	}
}
func testApiBasics(client DistCronClient, handle *JobHandle, t *testing.T) {
	if status, err := client.StopJob(context.Background(), handle); err != nil {
		glog.Info("StopJob", status, err, handle)
		t.Error(err)
	} else {
		glog.Info("StopJob", status, err, handle)
		t.Log(status)
	}

	if status, err := client.GetJobStatus(context.Background(), handle); err != nil {
		glog.Info("GetJobStatus", status, err, handle)
		t.Error(err)
	} else {
		glog.Info("GetJobStatus", status, err, handle)
		t.Log(status)
	}

	output, err := client.GetJobOutput(context.Background(), handle)
	if err != nil {
		t.Error(err)
		return
	}
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
