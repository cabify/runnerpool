// Code generated by mockery v1.0.0. DO NOT EDIT.

package runnerpoolmock

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// Worker is an autogenerated mock type for the Worker type
type Worker struct {
	mock.Mock
}

// Release provides a mock function with given fields:
func (_m *Worker) Release() {
	_m.Called()
}

// Run provides a mock function with given fields: _a0
func (_m *Worker) Run(_a0 func(context.Context)) {
	_m.Called(_a0)
}