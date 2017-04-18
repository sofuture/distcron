package dc

import (
	"fmt"
	"github.com/docker/leadership"
	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/golang/glog"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/assert"
	"os"
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

func makeTestNode(t *testing.T, name string, port int) *Node {
	leaderChannel := make(chan bool, 12)

	db, err := testDB()
	assert.NoError(t, err)

	config := &ClusterConfig{
		NodeName: name,
		BindAddr: "127.0.0.1",
		BindPort: port,
	}

	telemetryChannel := mkTelemetryChan(name, t)
	node, err := NewClusterNode(config, db, leaderChannel, telemetryChannel)
	assert.NoError(t, err)

	go printEvents(name, t, node)
	go func() {
		for leader := range leaderChannel {
			t.Logf("[%s] : leader %v", name, leader)
		}
	}()

	node.Start()

	return node
}

func makeTestCluster(t *testing.T) (*Node, *Node) {
	node1 := makeTestNode(t, "one", 5001)
	node2 := makeTestNode(t, "two", 5002)

	time.Sleep(time.Second * 1)

	if _, err := node2.serf.Join([]string{"127.0.0.1:5001"}, true); err != nil {
		t.Error(err)
	}

	return node1, node2
}

func TestOneNode(t *testing.T) {
	makeTestNode(t, fmt.Sprintf("%d", os.Getpid()), os.Getpid())
	time.Sleep(time.Minute * 10)
}

func TestBasicCluster(t *testing.T) {
	node1, node2 := makeTestCluster(t)

	time.Sleep(time.Second * 1)
	if len(node1.serf.Members()) != 2 || len(node2.serf.Members()) != 2 {
		t.Error("Nodes did not find each other")
	}

	time.Sleep(time.Minute * 1)
	node1.serf.Shutdown()
	node2.serf.Shutdown()
}

func TestClusterOwnershipChange(t *testing.T) {
	node1, node2 := makeTestCluster(t)
	for !node1.IsLeader() && !node2.IsLeader() {
		time.Sleep(time.Second)
	}
	leaderName, _, err := node1.GetLeader()
	assert.NoError(t, err)

	var leader, follower *Node
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

func printEvents(name string, t *testing.T, node *Node) {
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
