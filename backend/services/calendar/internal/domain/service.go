package domain

import (
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
)

// Service is the calendar domain. Its policy reads decide every request; its
// policy writes are the only way the graph changes, so the EPP invalidates
// every replica after each one.
type Service struct {
	store       Store
	policyRead  policypb.PolicyReadServiceClient
	policyWrite policypb.PolicyWriteServiceClient
	events      realtime.Emitter
}

// NewService builds the domain over its dependencies. events may be nil.
func NewService(st Store, read policypb.PolicyReadServiceClient, write policypb.PolicyWriteServiceClient, events realtime.Emitter) *Service {
	return &Service{store: st, policyRead: read, policyWrite: write, events: events}
}

// emit announces a committed change. Safe without an emitter.
func (s *Service) emit(e realtime.Event) {
	if s.events != nil {
		s.events.Emit(e)
	}
}
