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

func (api *apiService) Start(ctx context.Context, listenTo string) error {
	lc, err := net.Listen("tcp", listenTo)
	if err != nil {
		glog.Errorf("can't listen: %v", err)
		return err
	}

	err = api.rpcInfo.SetRpcForNode(api.rpcInfo.GetNodeName(), listenTo)
	if err != nil {
		glog.Error(err)
		return err
	}

	RegisterDistCronServer(api.server, api)
	go func() {
		glog.Error(api.server.Serve(lc))
	}()

	go func() {
		<-ctx.Done()
		api.server.GracefulStop()
	}()

	return nil
}

func (api *apiService) getLeaderRpc(ctx context.Context) (DistCronClient, error) {
	leaderNode, err := api.rpcInfo.GetLeaderNode()
	if err != nil {
		glog.Errorf("[%s] getLeaderRpc %v", api.rpcInfo.GetNodeName(), err)
		return nil, err
	}

	leaderRpc, err := api.rpcInfo.GetRpcForNode(ctx, leaderNode)
	if err != nil {
		glog.Errorf("[%s] getLeaderRpc %v", api.rpcInfo.GetNodeName(), err)
		return nil, err
	}

	return leaderRpc, nil
}

func (api *apiService) getRpcFromHandle(ctx context.Context, jh *JobHandle) (*jobHandle, DistCronClient, error) {
	handle, err := jh.ToInternal()
	if err != nil {
		return nil, nil, err
	}

	if handle.Node == api.rpcInfo.GetNodeName() {
		return handle, nil, nil
	}

	rpc, err := api.rpcInfo.GetRpcForNode(ctx, handle.Node)
	return handle, rpc, err
}

func (api *apiService) RunJob(ctx context.Context, job *Job) (*JobHandle, error) {
	if api.rpcInfo.IsLeader() {
		handle, err := api.dispatcher.NewJob(ctx, job)
		if err != nil {
			glog.Errorf("[%s] RunJob %v", api.rpcInfo.GetNodeName(), err)
		}
		return handle, err
	}

	leaderRpc, err := api.getLeaderRpc(ctx)
	if err != nil {
		glog.Errorf("[%s] RunJob %v", api.rpcInfo.GetNodeName(), err)
		return nil, InternalError
	}

	return leaderRpc.RunJob(ctx, job)
}

// RunJobOnThisNode is internal API method to execute job on the node which received the call
func (api *apiService) RunJobOnThisNode(ctx context.Context, job *Job) (*JobHandle, error) {
	cid, err := api.runner.RunJob(ctx, job)
	if err != nil {
		glog.Errorf("[%s] RunJobOnThisNode %v", api.rpcInfo.GetNodeName(), err)
		return nil, err
	}

	return (&jobHandle{CID: cid, Node: api.rpcInfo.GetNodeName()}).Handle(), nil
}

func (api *apiService) StopJob(ctx context.Context, jh *JobHandle) (*JobStatus, error) {
	handle, nodeRpc, err := api.getRpcFromHandle(ctx, jh)
	if err != nil {
		glog.Errorf("[%s] StopJob %v", api.rpcInfo.GetNodeName(), err)
		return nil, InternalError
	}

	if nodeRpc != nil {
		return nodeRpc.StopJob(ctx, jh)
	}

	return api.runner.StopJob(ctx, handle.CID)
}

func (api *apiService) GetJobStatus(ctx context.Context, jh *JobHandle) (*JobStatus, error) {
	handle, nodeRpc, err := api.getRpcFromHandle(ctx, jh)
	if err != nil {
		glog.Errorf("[%s] GetJobStatus %v", api.rpcInfo.GetNodeName(), err)
		return nil, InternalError
	}

	if nodeRpc != nil {
		return nodeRpc.GetJobStatus(ctx, jh)
	}

	return api.runner.GetJobStatus(ctx, handle.CID)
}

func (api *apiService) streamLocalJobOutput(handle *jobHandle, stream DistCron_GetJobOutputServer) error {
	err := api.runner.GetJobOutput(stream.Context(), handle.CID, func(data []byte) error {
		return stream.Send(&Output{data})
	})
	if err != nil {
		glog.Error(err)
		return InternalError
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
	handle, nodeRpc, err := api.getRpcFromHandle(stream.Context(), jh)
	if err != nil {
		glog.Error(err)
		return InternalError
	}

	if nodeRpc != nil { // forward to the node where it was executed
		return streamJobOutputFromNode(jh, nodeRpc, stream)
	}

	return api.streamLocalJobOutput(handle, stream)
}
