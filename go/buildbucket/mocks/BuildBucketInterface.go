// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	context "context"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"

	mock "github.com/stretchr/testify/mock"
)

// BuildBucketInterface is an autogenerated mock type for the BuildBucketInterface type
type BuildBucketInterface struct {
	mock.Mock
}

// GetBuild provides a mock function with given fields: ctx, buildId
func (_m *BuildBucketInterface) GetBuild(ctx context.Context, buildId int64) (*buildbucketpb.Build, error) {
	ret := _m.Called(ctx, buildId)

	var r0 *buildbucketpb.Build
	if rf, ok := ret.Get(0).(func(context.Context, int64) *buildbucketpb.Build); ok {
		r0 = rf(ctx, buildId)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*buildbucketpb.Build)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, buildId)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTrybotsForCL provides a mock function with given fields: ctx, issue, patchset, gerritUrl
func (_m *BuildBucketInterface) GetTrybotsForCL(ctx context.Context, issue int64, patchset int64, gerritUrl string) ([]*buildbucketpb.Build, error) {
	ret := _m.Called(ctx, issue, patchset, gerritUrl)

	var r0 []*buildbucketpb.Build
	if rf, ok := ret.Get(0).(func(context.Context, int64, int64, string) []*buildbucketpb.Build); ok {
		r0 = rf(ctx, issue, patchset, gerritUrl)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*buildbucketpb.Build)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64, int64, string) error); ok {
		r1 = rf(ctx, issue, patchset, gerritUrl)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Search provides a mock function with given fields: ctx, pred
func (_m *BuildBucketInterface) Search(ctx context.Context, pred *buildbucketpb.BuildPredicate) ([]*buildbucketpb.Build, error) {
	ret := _m.Called(ctx, pred)

	var r0 []*buildbucketpb.Build
	if rf, ok := ret.Get(0).(func(context.Context, *buildbucketpb.BuildPredicate) []*buildbucketpb.Build); ok {
		r0 = rf(ctx, pred)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*buildbucketpb.Build)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *buildbucketpb.BuildPredicate) error); ok {
		r1 = rf(ctx, pred)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
