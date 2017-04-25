package dc

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"net"

	"github.com/golang/glog"
	"github.com/hashicorp/serf/serf"
	"golang.org/x/net/context"
)

// log levels
const cLogDebug = 0

// 1 for development and testing
// 64-256 for production
const cChanBuffer = 1

type Node interface {
	// GetName returns this node logical name, must be unique in the cluster
	GetName() string
	// IsLeader returns true if this node is a current cluster leader
	IsLeader() bool
	// GetLeader returns current cluster leader node
	GetLeader() (name string, addr net.IP, err error)
	SerfMembers() []serf.Member
	SerfMembersCount() int
	// Starts node service, use context to terminate
	Start(context.Context)
	// Join other known nodes in the cluster, should be in form addr:port
	Join(addr []string) error
}

type RpcInfo interface {
	// current node name
	GetNodeName() string
	// is current node a leader
	IsLeader() bool
	// returns leader node name
	GetLeaderNode() (string, error)
	// stores RPC address for a given node in some distributed storage
	SetRpcForNode(node, addr string) error
	// given a node name, provides RPC client instance
	GetRpcForNode(ctx context.Context, node string) (DistCronClient, error)
}

type Dispatcher interface {
	// Start dispatcher service. Use context to stop the service
	Start(context.Context)
	// NewJob places a job on next available node, taking care of resources
	// if no node is available, NoNodesAvailableError is returned
	NewJob(ctx context.Context, job *Job) (*JobHandle, error)
}

type DataCopyFn func(data []byte) error

type Runner interface {
	// RunJob spawns new job, returns internal container ID
	RunJob(ctx context.Context, job *Job) (cid string, err error)
	// GetJobStatus returns job status as reported by docker daemon, format is unspecified
	GetJobStatus(ctx context.Context, cid string) (status *JobStatus, err error)
	// StopJob requests docker daemon to stop a job and returns current status
	StopJob(ctx context.Context, cid string) (status *JobStatus, err error)
	// GetJobOutput provides streaming of container most recent output
	GetJobOutput(ctx context.Context, cid string, fn DataCopyFn) (err error)
}

/*
 * simple and insecure : job handle is node name + container ID
 * in real life should be GUID and stored in DB
 *
 * provides mapping to PB JobHandle message
 */
type jobHandle struct {
	Node, CID string
}

func (h *jobHandle) Handle() *JobHandle {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	enc.Encode(h)
	return &JobHandle{base64.StdEncoding.EncodeToString(buf.Bytes())}
}

func (handle *JobHandle) ToInternal() (*jobHandle, error) {
	var hdl jobHandle
	data, err := base64.StdEncoding.DecodeString(handle.Handle)
	if err != nil {
		glog.Error(handle, err)
		return nil, err
	}
	if err := gob.NewDecoder(bytes.NewBuffer(data)).Decode(&hdl); err != nil {
		glog.Error(handle, err)
		return nil, err
	} else {
		return &hdl, nil
	}
}

// no nodes currently available. this may be a transient error when cluster is rebalancing
var NoNodesAvailableError = fmt.Errorf("E_NO_NODES_AVAILABLE")

// don't want to expose details
var InternalError = fmt.Errorf("E_INTERNAL_ERROR")

// deadline exceeded for request
var RequestTimeoutError = fmt.Errorf("E_REQUEST_TIMEOUT")
