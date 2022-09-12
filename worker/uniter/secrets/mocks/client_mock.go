// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/juju/juju/worker/uniter/secrets (interfaces: SecretsClient)

// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	secrets "github.com/juju/juju/core/secrets"
	watcher "github.com/juju/juju/core/watcher"
)

// MockSecretsClient is a mock of SecretsClient interface.
type MockSecretsClient struct {
	ctrl     *gomock.Controller
	recorder *MockSecretsClientMockRecorder
}

// MockSecretsClientMockRecorder is the mock recorder for MockSecretsClient.
type MockSecretsClientMockRecorder struct {
	mock *MockSecretsClient
}

// NewMockSecretsClient creates a new mock instance.
func NewMockSecretsClient(ctrl *gomock.Controller) *MockSecretsClient {
	mock := &MockSecretsClient{ctrl: ctrl}
	mock.recorder = &MockSecretsClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSecretsClient) EXPECT() *MockSecretsClientMockRecorder {
	return m.recorder
}

// GetConsumerSecretsRevisionInfo mocks base method.
func (m *MockSecretsClient) GetConsumerSecretsRevisionInfo(arg0 string, arg1 []string) (map[string]secrets.SecretRevisionInfo, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetConsumerSecretsRevisionInfo", arg0, arg1)
	ret0, _ := ret[0].(map[string]secrets.SecretRevisionInfo)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetConsumerSecretsRevisionInfo indicates an expected call of GetConsumerSecretsRevisionInfo.
func (mr *MockSecretsClientMockRecorder) GetConsumerSecretsRevisionInfo(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetConsumerSecretsRevisionInfo", reflect.TypeOf((*MockSecretsClient)(nil).GetConsumerSecretsRevisionInfo), arg0, arg1)
}

// SecretMetadata mocks base method.
func (m *MockSecretsClient) SecretMetadata(arg0 secrets.Filter) ([]secrets.SecretOwnerMetadata, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SecretMetadata", arg0)
	ret0, _ := ret[0].([]secrets.SecretOwnerMetadata)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// SecretMetadata indicates an expected call of SecretMetadata.
func (mr *MockSecretsClientMockRecorder) SecretMetadata(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SecretMetadata", reflect.TypeOf((*MockSecretsClient)(nil).SecretMetadata), arg0)
}

// WatchConsumedSecretsChanges mocks base method.
func (m *MockSecretsClient) WatchConsumedSecretsChanges(arg0 string) (watcher.StringsWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchConsumedSecretsChanges", arg0)
	ret0, _ := ret[0].(watcher.StringsWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchConsumedSecretsChanges indicates an expected call of WatchConsumedSecretsChanges.
func (mr *MockSecretsClientMockRecorder) WatchConsumedSecretsChanges(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchConsumedSecretsChanges", reflect.TypeOf((*MockSecretsClient)(nil).WatchConsumedSecretsChanges), arg0)
}

// WatchObsoleteRevisions mocks base method.
func (m *MockSecretsClient) WatchObsoleteRevisions(arg0 string) (watcher.StringsWatcher, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WatchObsoleteRevisions", arg0)
	ret0, _ := ret[0].(watcher.StringsWatcher)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// WatchObsoleteRevisions indicates an expected call of WatchObsoleteRevisions.
func (mr *MockSecretsClientMockRecorder) WatchObsoleteRevisions(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WatchObsoleteRevisions", reflect.TypeOf((*MockSecretsClient)(nil).WatchObsoleteRevisions), arg0)
}