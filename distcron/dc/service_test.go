package dc

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func TestBasicService(t *testing.T) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()

	cfg := []ClusterConfig{
		ClusterConfig{
			NodeName: "A",
			BindAddr: "127.0.0.1",
			BindPort: 5006,
		},
		ClusterConfig{
			NodeName: "B",
			BindAddr: "127.0.0.1",
			BindPort: 5007,
		},
		ClusterConfig{
			NodeName: "C",
			BindAddr: "127.0.0.1",
			BindPort: 5008,
		}}

	svc := mkServiceCluster(ctx, t, cfg)

	for len(svc) >= 2 {
		leader := waitForLeader(t, svc)
		t.Logf("%s is new leader", leader)
		testAllApiBasics(ctx, t, svc)
		svc = killLeader(t, svc)
	}
}

func mkApiClient(dialTo string) (DistCronClient, error) {
	if conn, err := grpc.Dial(dialTo, grpc.WithInsecure()); err != nil {
		return nil, err
	} else {
		return NewDistCronClient(conn), nil
	}
}

type cronSvc struct {
	cfg     ClusterConfig
	stop    context.CancelFunc
	service *DistCron
}

func mkServiceCluster(ctx context.Context, t *testing.T, config []ClusterConfig) (svc []cronSvc) {
	dbHosts := []string{"consul:8500"}

	if len(config) < 2 {
		require.Fail(t, "Cluster requires at least 2 nodes")
	}

	svc = make([]cronSvc, len(config))
	basePort := 5555

	for i, _ := range config {
		var c context.Context
		var err error

		svc[i].cfg = config[i]
		c, svc[i].stop = context.WithCancel(ctx)
		svc[i].service, err = NewDistCron(c, &config[i], dbHosts, fmt.Sprintf(":%d", basePort+i))
		require.NoError(t, err)
	}

	joinTo := []string{fmt.Sprintf("%s:%d", config[0].BindAddr, config[0].BindPort)}
	for i := 1; i < len(config); i++ {
		require.NoError(t, svc[i].service.JoinTo(joinTo))
	}

	return svc
}

func waitForLeader(t *testing.T, svc []cronSvc) string {
	for retry := 0; retry < 20; retry++ {
		var leaderSvc *cronSvc
		for _, s := range svc {
			if s.service.IsLeader() {
				if leaderSvc != nil {
					require.Fail(t, "Multiple nodes report being leader : %v and %v", leaderSvc.service.GetNodeName(), s.service.GetNodeName())
					return ""
				}
				leaderSvc = &s
			} else {
				t.Logf("%s is NOT a leader", s.service.GetNodeName())
			}
		}
		if leaderSvc != nil {
			return leaderSvc.service.GetNodeName()
		}
		time.Sleep(time.Second * 1)
	}
	require.Fail(t, "Timed out waiting for cluster leader")
	return ""
}

// kills current leader and returns truncated service list
func killLeader(t *testing.T, svc []cronSvc) []cronSvc {
	for i, s := range svc {
		if !s.service.IsLeader() {
			continue
		}
		t.Logf("Killing leader %s", s.service.GetNodeName())
		s.stop()
		return append(svc[:i], svc[i+1:]...)
	}
	require.Fail(t, "No leader in cluster")
	return nil
}

func testAllApiBasics(ctx context.Context, t *testing.T, svc []cronSvc) {
	for _, s := range svc {
		client, err := s.service.GetRpcForNode(ctx, s.cfg.NodeName)
		require.NoError(t, err)
		testApiBasics(ctx, client, t)
	}
}

func testApiBasics(ctx context.Context, client DistCronClient, t *testing.T) {
	err := NoNodesAvailableError
	var handle *JobHandle
	handle, err = client.RunJob(ctx, &Job{
		ContainerName: "hello-world",
		CpuLimit:      1,
		MemLimitMb:    500,
	})
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
