package dc

import (
	"github.com/docker/leadership"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/golang/glog"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func testDB() (*DB, error) {
	//return ConnectDB([]string{"172.17.8.101:2379"})
	return ConnectDB([]string{"127.0.0.1:8500"})
}

func mkTelemetryChan(label string, t *testing.T) chan *TelemetryInfo {
	ch := make(chan *TelemetryInfo)
	go func() {
		for evt := range ch {
			t.Logf("[%s] %v", label, evt)
		}
	}()
	return ch
}

func makeTestNode(t *testing.T, name string, port int) Node {
	leaderChannel := make(chan bool, 12)

	db, err := testDB()
	require.NoError(t, err)

	config := &ClusterConfig{
		NodeName: name,
		BindAddr: "127.0.0.1",
		BindPort: port,
	}

	telemetryChannel := mkTelemetryChan(name, t)
	node, err := NewClusterNode(config, db, leaderChannel, telemetryChannel)
	require.NoError(t, err)

	go printEvents(name, t, node.(*cronNode))
	go func() {
		for leader := range leaderChannel {
			t.Logf("[%s] : leader %v", name, leader)
		}
	}()

	node.Start()

	return node
}

func makeTestCluster(t *testing.T) (Node, Node) {
	node1 := makeTestNode(t, "one", 5001)
	node2 := makeTestNode(t, "two", 5002)

	time.Sleep(time.Second * 1)

	if err := node2.Join([]string{"127.0.0.1:5001"}); err != nil {
		t.Error(err)
	}

	return node1, node2
}

func TestBasicCluster(t *testing.T) {
	node1, node2 := makeTestCluster(t)

	time.Sleep(time.Second * 1)
	if node1.SerfMembersCount() != 2 || node2.SerfMembersCount() != 2 {
		t.Error("Nodes did not find each other")
	}

	node1.Stop()
	node2.Stop()
}

func TestClusterOwnershipChange(t *testing.T) {
	node1, node2 := makeTestCluster(t)
	for !node1.IsLeader() && !node2.IsLeader() {
		time.Sleep(time.Second)
	}
	leaderName, _, err := node1.GetLeader()
	require.NoError(t, err)

	var leader, follower Node
	if leaderName == "one" {
		leader, follower = node1, node2
	} else {
		leader, follower = node2, node1
	}

	leader.Stop()
	time.Sleep(time.Second * 20)
	for !follower.IsLeader() {
		time.Sleep(time.Second)
	}

	t.Log("New leader")
}

func printEvents(name string, t *testing.T, node *cronNode) {
	for e := range node.serfChannel {
		if me, ok := e.(serf.MemberEvent); ok {
			t.Logf("%s : %s : %v", name, e, me.Members)
		} else {
			t.Logf("%s : %s", name, e)
		}
	}
	t.Log("Channel closed")
}

func TestLeadership(t *testing.T) {
	c1 := mkCandidate("one", "127.0.0.1:8500")
	c2 := mkCandidate("two", "127.0.0.1:8500")
	time.Sleep(time.Second * 5)
	if c1.IsLeader() {
		c1.Stop()
	} else {
		c2.Stop()
	}
	time.Sleep(time.Second * 5)
}

func mkCandidate(name, dbHost string) *leadership.Candidate {
	store, _ := libkv.NewStore(store.CONSUL, []string{dbHost},
		&store.Config{Bucket: "distcron"})

	candidate := leadership.NewCandidate(store, CLeadershipKey,
		name, cDefaultLeaderLock)

	go func() {
		electionChan, errorChan := candidate.RunForElection()
		for {
			select {
			case leader := <-electionChan:
				glog.Infof("[%s] leader=%v", name, leader)
			case err := <-errorChan:
				glog.Errorf("[%s] error=%v", name, err)
				electionChan, errorChan = candidate.RunForElection()
			}
		}
	}()

	return candidate
}
