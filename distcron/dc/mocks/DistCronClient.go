package mocks

import context "golang.org/x/net/context"
import dc "distcron/dc"
import grpc "google.golang.org/grpc"
import mock "github.com/stretchr/testify/mock"

// DistCronClient is an autogenerated mock type for the DistCronClient type
type DistCronClient struct {
	mock.Mock
}

// GetJobOutput provides a mock function with given fields: ctx, in, opts
func (_m *DistCronClient) GetJobOutput(ctx context.Context, in *dc.JobHandle, opts ...grpc.CallOption) (dc.DistCron_GetJobOutputClient, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 dc.DistCron_GetJobOutputClient
	if rf, ok := ret.Get(0).(func(context.Context, *dc.JobHandle, ...grpc.CallOption) dc.DistCron_GetJobOutputClient); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(dc.DistCron_GetJobOutputClient)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *dc.JobHandle, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetJobStatus provides a mock function with given fields: ctx, in, opts
func (_m *DistCronClient) GetJobStatus(ctx context.Context, in *dc.JobHandle, opts ...grpc.CallOption) (*dc.JobStatus, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *dc.JobStatus
	if rf, ok := ret.Get(0).(func(context.Context, *dc.JobHandle, ...grpc.CallOption) *dc.JobStatus); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dc.JobStatus)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *dc.JobHandle, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RunJob provides a mock function with given fields: ctx, in, opts
func (_m *DistCronClient) RunJob(ctx context.Context, in *dc.Job, opts ...grpc.CallOption) (*dc.JobHandle, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *dc.JobHandle
	if rf, ok := ret.Get(0).(func(context.Context, *dc.Job, ...grpc.CallOption) *dc.JobHandle); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dc.JobHandle)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *dc.Job, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RunJobOnThisNode provides a mock function with given fields: ctx, in, opts
func (_m *DistCronClient) RunJobOnThisNode(ctx context.Context, in *dc.Job, opts ...grpc.CallOption) (*dc.JobHandle, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *dc.JobHandle
	if rf, ok := ret.Get(0).(func(context.Context, *dc.Job, ...grpc.CallOption) *dc.JobHandle); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dc.JobHandle)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *dc.Job, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StopJob provides a mock function with given fields: ctx, in, opts
func (_m *DistCronClient) StopJob(ctx context.Context, in *dc.JobHandle, opts ...grpc.CallOption) (*dc.JobStatus, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, in)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *dc.JobStatus
	if rf, ok := ret.Get(0).(func(context.Context, *dc.JobHandle, ...grpc.CallOption) *dc.JobStatus); ok {
		r0 = rf(ctx, in, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dc.JobStatus)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *dc.JobHandle, ...grpc.CallOption) error); ok {
		r1 = rf(ctx, in, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
