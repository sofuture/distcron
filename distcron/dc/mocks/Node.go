package mocks

import context "golang.org/x/net/context"

import mock "github.com/stretchr/testify/mock"
import net "net"
import serf "github.com/hashicorp/serf/serf"

// Node is an autogenerated mock type for the Node type
type Node struct {
	mock.Mock
}

// GetLeader provides a mock function with given fields:
func (_m *Node) GetLeader() (string, net.IP, error) {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	var r1 net.IP
	if rf, ok := ret.Get(1).(func() net.IP); ok {
		r1 = rf()
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(net.IP)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func() error); ok {
		r2 = rf()
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetName provides a mock function with given fields:
func (_m *Node) GetName() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// IsLeader provides a mock function with given fields:
func (_m *Node) IsLeader() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Join provides a mock function with given fields: addr
func (_m *Node) Join(addr []string) error {
	ret := _m.Called(addr)

	var r0 error
	if rf, ok := ret.Get(0).(func([]string) error); ok {
		r0 = rf(addr)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SerfMembers provides a mock function with given fields:
func (_m *Node) SerfMembers() []serf.Member {
	ret := _m.Called()

	var r0 []serf.Member
	if rf, ok := ret.Get(0).(func() []serf.Member); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]serf.Member)
		}
	}

	return r0
}

// SerfMembersCount provides a mock function with given fields:
func (_m *Node) SerfMembersCount() int {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// Start provides a mock function with given fields: _a0
func (_m *Node) Start(_a0 context.Context) {
	_m.Called(_a0)
}