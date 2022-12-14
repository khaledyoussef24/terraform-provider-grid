// Code generated by MockGen. DO NOT EDIT.
// Source: ../../../pkg/subi/dev_manager.go

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	substrate "github.com/threefoldtech/substrate-client"
	subi "github.com/threefoldtech/terraform-provider-grid/pkg/subi"
)

// MockManager is a mock of Manager interface.
type MockManager struct {
	ctrl     *gomock.Controller
	recorder *MockManagerMockRecorder
}

// MockManagerMockRecorder is the mock recorder for MockManager.
type MockManagerMockRecorder struct {
	mock *MockManager
}

// NewMockManager creates a new mock instance.
func NewMockManager(ctrl *gomock.Controller) *MockManager {
	mock := &MockManager{ctrl: ctrl}
	mock.recorder = &MockManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockManager) EXPECT() *MockManagerMockRecorder {
	return m.recorder
}

// Raw mocks base method.
func (m *MockManager) Raw() (substrate.Conn, substrate.Meta, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Raw")
	ret0, _ := ret[0].(substrate.Conn)
	ret1, _ := ret[1].(substrate.Meta)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// Raw indicates an expected call of Raw.
func (mr *MockManagerMockRecorder) Raw() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Raw", reflect.TypeOf((*MockManager)(nil).Raw))
}

// Substrate mocks base method.
func (m *MockManager) Substrate() (*substrate.Substrate, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Substrate")
	ret0, _ := ret[0].(*substrate.Substrate)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Substrate indicates an expected call of Substrate.
func (mr *MockManagerMockRecorder) Substrate() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Substrate", reflect.TypeOf((*MockManager)(nil).Substrate))
}

// SubstrateExt mocks base method.
func (m *MockManager) SubstrateExt() (subi.SubstrateExt, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubstrateExt")
	ret0, _ := ret[0].(subi.SubstrateExt)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SubstrateExt indicates an expected call of SubstrateExt.
func (mr *MockManagerMockRecorder) SubstrateExt() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubstrateExt", reflect.TypeOf((*MockManager)(nil).SubstrateExt))
}
