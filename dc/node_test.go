package dc

import (
	"fmt"
	"github.com/hashicorp/serf/serf"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
	"time"
)

func testDB() (*DB, error) {
	return ConnectDB([]string{"172.17.8.101:2379"})
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

func TestCluster(t *testing.T) {
	node1, node2 := makeTestCluster(t)

	time.Sleep(time.Second * 1)
	if len(node1.serf.Members()) != 2 || len(node2.serf.Members()) != 2 {
		t.Error("Nodes did not find each other")
	}

	time.Sleep(time.Minute * 1)
	node1.serf.Shutdown()
	node2.serf.Shutdown()
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
