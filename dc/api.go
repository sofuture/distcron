package dc

import (
	"io"
	"net"

	"github.com/golang/glog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type apiService struct {
	runner     Runner
	dispatcher Dispatcher
	rpcInfo    RpcInfo
	server     *grpc.Server
}

/*
 * API calls get forwarded to either relevant node or to a leader
 * No authentication for sample implementation
 */

func NewApiServer(runner Runner, dispatcher Dispatcher, rpcInfo RpcInfo) (*apiService, error) {
	apiSvc := &apiService{
		runner:     runner,
		dispatcher: dispatcher,
		server:     grpc.NewServer(),
		rpcInfo:    rpcInfo,
	}

	return apiSvc, nil
}

func (api *apiService) Start(listenTo string) error {
	lc, err := net.Listen("tcp", listenTo)
	if err != nil {
		glog.Errorf("can't listen: %v", err)
		return err
	}

	RegisterDistCronServer(api.server, api)
	go func() {
		glog.Error(api.server.Serve(lc))
		glog.Error(lc)
	}()

	api.rpcInfo.SetRpcForNode(api.rpcInfo.GetNodeName(), listenTo)

	return nil
}

func (api *apiService) Stop() {
	api.server.GracefulStop()
}

func (api *apiService) getLeaderRpc(ctx context.Context) (DistCronClient, error) {
	if leaderNode, err := api.rpcInfo.GetLeaderNode(); err != nil {
		glog.Error(err)
		return nil, err
	} else if leaderRpc, err := api.rpcInfo.GetRpcForNode(ctx, leaderNode); err != nil {
		glog.Error(err)
		return nil, err
	} else {
		return leaderRpc, nil
	}
}

func (api *apiService) getRpcFromHandle(ctx context.Context, jh *JobHandle) (*jobHandle, DistCronClient, error) {
	if handle, err := jh.ToInternal(); err != nil {
		return nil, nil, err
	} else if handle.Node == api.rpcInfo.GetNodeName() {
		return handle, nil, nil
	} else {
		rpc, err := api.rpcInfo.GetRpcForNode(ctx, handle.Node)
		return handle, rpc, err
	}
}

func (api *apiService) RunJob(ctx context.Context, job *Job) (*JobHandle, error) {
	if api.rpcInfo.IsLeader() {
		glog.Error(api)
		return api.dispatcher.NewJob(ctx, job)
	} else if leaderRpc, err := api.getLeaderRpc(ctx); err != nil {
		return nil, EInternalError
	} else {
		return leaderRpc.RunJob(ctx, job)
	}

}

// internal API method to execute job on specific node
func (api *apiService) RunJobOnThisNode(ctx context.Context, job *Job) (*JobHandle, error) {
	if cid, err := api.runner.RunJob(ctx, job); err != nil {
		return nil, err
	} else {
		return (&jobHandle{CID: cid, Node: api.rpcInfo.GetNodeName()}).Handle(), nil
	}
}

func (api *apiService) StopJob(ctx context.Context, jh *JobHandle) (*JobStatus, error) {
	if handle, nodeRpc, err := api.getRpcFromHandle(ctx, jh); err != nil {
		glog.Error(err)
		return nil, EInternalError
	} else if nodeRpc != nil {
		return nodeRpc.StopJob(ctx, jh)
	} else {
		return api.runner.StopJob(ctx, handle.CID)
	}
}

func (api *apiService) GetJobStatus(ctx context.Context, jh *JobHandle) (*JobStatus, error) {
	if handle, nodeRpc, err := api.getRpcFromHandle(ctx, jh); err != nil {
		glog.Error(err)
		return nil, EInternalError
	} else if nodeRpc != nil {
		return nodeRpc.GetJobStatus(ctx, jh)
	} else {
		return api.runner.GetJobStatus(ctx, handle.CID)
	}
}

func (api *apiService) streamLocalJobOutput(handle *jobHandle, stream DistCron_GetJobOutputServer) error {
	err := api.runner.GetJobOutput(stream.Context(), handle.CID, func(data []byte) error {
		return stream.Send(&Output{data})
	})
	if err != nil {
		glog.Error(err)
		return EInternalError
	}
	return nil
}

func streamJobOutputFromNode(jh *JobHandle, nodeRpc DistCronClient, stream DistCron_GetJobOutputServer) error {
	ctx := stream.Context()

	output, err := nodeRpc.GetJobOutput(ctx, jh)
	if err != nil {
		glog.Error(err)
		return err
	}

	for {
		select {
		case <-ctx.Done():
			break
		default:
		}

		data, err := output.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			glog.Error(err)
			return err
		}
		if err = stream.Send(data); err != nil {
			glog.Error(err)
			return err
		}
	}

	return nil
}

func (api *apiService) GetJobOutput(jh *JobHandle, stream DistCron_GetJobOutputServer) error {
	if handle, nodeRpc, err := api.getRpcFromHandle(stream.Context(), jh); err != nil {
		glog.Error(err)
		return EInternalError
	} else if nodeRpc != nil { // forward to the node where it was executed
		return streamJobOutputFromNode(jh, nodeRpc, stream)
	} else {
		return api.streamLocalJobOutput(handle, stream)
	}
}
