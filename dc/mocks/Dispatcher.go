package mocks

import context "golang.org/x/net/context"
import dc "distcron/dc"
import mock "github.com/stretchr/testify/mock"

// Dispatcher is an autogenerated mock type for the Dispatcher type
type Dispatcher struct {
	mock.Mock
}

// NewJob provides a mock function with given fields: ctx, job
func (_m *Dispatcher) NewJob(ctx context.Context, job *dc.Job) (*dc.JobHandle, error) {
	ret := _m.Called(ctx, job)

	var r0 *dc.JobHandle
	if rf, ok := ret.Get(0).(func(context.Context, *dc.Job) *dc.JobHandle); ok {
		r0 = rf(ctx, job)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dc.JobHandle)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *dc.Job) error); ok {
		r1 = rf(ctx, job)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Start provides a mock function with given fields:
func (_m *Dispatcher) Start() {
	_m.Called()
}

// Stop provides a mock function with given fields:
func (_m *Dispatcher) Stop() {
	_m.Called()
}
