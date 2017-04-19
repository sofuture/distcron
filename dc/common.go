package dc

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"github.com/golang/glog"
	"github.com/hashicorp/serf/serf"
	"net"
)

// log levels
const cLogDebug = 0

// 1 for development and testing
// 64-256 for production
const cChanBuffer = 10

type Node interface {
	GetName() string
	IsLeader() bool
	GetLeader() (name string, addr net.IP, err error)
	SerfMembers() []serf.Member
	SerfMembersCount() int
	Start()
	Stop()
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
	GetRpcForNode(node string) (DistCronClient, error)
}

type Dispatcher interface {
	Start()
	Stop()
	NewJob(job *Job) (*JobHandle, error)
}

type DataCopyFn func(data []byte) error

type Runner interface {
	RunJob(job *Job) (cid string, err error)
	GetJobStatus(cid string) (status *JobStatus, err error)
	StopJob(cid string) (status *JobStatus, err error)
	GetJobOutput(cid string, fn DataCopyFn) (err error)
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

// common errors returned by API and services
var ENoNodesAvailable = fmt.Errorf("E_NO_NODES_AVAILABLE")
var EInternalError = fmt.Errorf("E_INTERNAL_ERROR")
