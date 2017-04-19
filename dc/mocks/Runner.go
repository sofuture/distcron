package mocks

import dc "distcron/dc"
import mock "github.com/stretchr/testify/mock"

// Runner is an autogenerated mock type for the Runner type
type Runner struct {
	mock.Mock
}

// GetJobOutput provides a mock function with given fields: cid, fn
func (_m *Runner) GetJobOutput(cid string, fn dc.DataCopyFn) error {
	ret := _m.Called(cid, fn)

	var r0 error
	if rf, ok := ret.Get(0).(func(string, dc.DataCopyFn) error); ok {
		r0 = rf(cid, fn)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetJobStatus provides a mock function with given fields: cid
func (_m *Runner) GetJobStatus(cid string) (*dc.JobStatus, error) {
	ret := _m.Called(cid)

	var r0 *dc.JobStatus
	if rf, ok := ret.Get(0).(func(string) *dc.JobStatus); ok {
		r0 = rf(cid)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dc.JobStatus)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(cid)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RunJob provides a mock function with given fields: job
func (_m *Runner) RunJob(job *dc.Job) (string, error) {
	ret := _m.Called(job)

	var r0 string
	if rf, ok := ret.Get(0).(func(*dc.Job) string); ok {
		r0 = rf(job)
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*dc.Job) error); ok {
		r1 = rf(job)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StopJob provides a mock function with given fields: cid
func (_m *Runner) StopJob(cid string) (*dc.JobStatus, error) {
	ret := _m.Called(cid)

	var r0 *dc.JobStatus
	if rf, ok := ret.Get(0).(func(string) *dc.JobStatus); ok {
		r0 = rf(cid)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*dc.JobStatus)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(cid)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
