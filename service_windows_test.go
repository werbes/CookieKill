//go:build windows

package main

import (
	"context"
	"errors"
	"sync"
	"testing"

	"golang.org/x/sys/windows/svc"
)

type serviceResult struct {
	specific bool
	code     uint32
}

type serviceHarness struct {
	handler  *windowsService
	requests chan svc.ChangeRequest
	statuses chan svc.Status
	done     chan serviceResult
}

func startServiceHarness(t *testing.T, run func(context.Context, func()) error) *serviceHarness {
	t.Helper()
	h := &serviceHarness{
		handler:  &windowsService{run: run},
		requests: make(chan svc.ChangeRequest, 8),
		statuses: make(chan svc.Status, 32),
		done:     make(chan serviceResult, 1),
	}
	t.Cleanup(func() { close(h.requests) })
	go func() {
		specific, code := h.handler.Execute(nil, h.requests, h.statuses)
		h.done <- serviceResult{specific: specific, code: code}
	}()
	status := receiveWithin(t, h.statuses)
	if status.State != svc.StartPending || status.CheckPoint == 0 || status.WaitHint == 0 {
		t.Fatalf("invalid startup status: %+v", status)
	}
	return h
}

func (h *serviceHarness) expectState(t *testing.T, state svc.State) svc.Status {
	t.Helper()
	status := receiveWithin(t, h.statuses)
	if status.State != state {
		t.Fatalf("service state = %v, want %v", status.State, state)
	}
	return status
}

func TestWindowsServiceWaitsForReadinessAndGracefulStop(t *testing.T) {
	for _, command := range []struct {
		name string
		cmd  svc.Cmd
	}{{"Stop", svc.Stop}, {"Shutdown", svc.Shutdown}} {
		t.Run(command.name, func(t *testing.T) {
			allowReady := make(chan struct{})
			canceled := make(chan struct{})
			finish := make(chan struct{})
			release := sync.OnceFunc(func() { close(finish) })
			t.Cleanup(release)
			h := startServiceHarness(t, func(ctx context.Context, ready func()) error {
				select {
				case <-allowReady:
					ready()
				case <-ctx.Done():
				}
				<-ctx.Done()
				close(canceled)
				<-finish
				return nil
			})
			h.requests <- svc.ChangeRequest{Cmd: svc.Interrogate}
			h.expectState(t, svc.StartPending)
			close(allowReady)
			status := h.expectState(t, svc.Running)
			if status.Accepts != svc.AcceptStop|svc.AcceptShutdown {
				t.Fatalf("running service accepts = %v", status.Accepts)
			}
			h.requests <- svc.ChangeRequest{Cmd: command.cmd}
			status = h.expectState(t, svc.StopPending)
			if status.CheckPoint == 0 || status.WaitHint == 0 {
				t.Fatalf("shutdown omitted checkpoint/wait hint: %+v", status)
			}
			receiveWithin(t, canceled)
			// An interrogation must still be handled while persistence is blocked.
			h.requests <- svc.ChangeRequest{Cmd: svc.Interrogate}
			h.expectState(t, svc.StopPending)
			select {
			case <-h.done:
				t.Fatal("service exited before the application finished shutting down")
			default:
			}
			release()
			result := receiveWithin(t, h.done)
			if result.specific || result.code != 0 || h.handler.err != nil {
				t.Fatalf("graceful stop returned %+v, error %v", result, h.handler.err)
			}
		})
	}
}

func TestWindowsServiceStopBeforeReady(t *testing.T) {
	lateReady := make(chan struct{})
	finish := make(chan struct{})
	release := sync.OnceFunc(func() { close(finish) })
	t.Cleanup(release)
	h := startServiceHarness(t, func(ctx context.Context, ready func()) error {
		<-ctx.Done()
		ready()
		close(lateReady)
		<-finish
		return nil
	})
	h.requests <- svc.ChangeRequest{Cmd: svc.Stop}
	h.expectState(t, svc.StopPending)
	receiveWithin(t, lateReady)
	h.requests <- svc.ChangeRequest{Cmd: svc.Interrogate}
	h.expectState(t, svc.StopPending)
	release()
	result := receiveWithin(t, h.done)
	if result.specific || result.code != 0 {
		t.Fatalf("graceful startup cancellation returned %+v", result)
	}
	for len(h.statuses) > 0 {
		if status := <-h.statuses; status.State == svc.Running {
			t.Fatal("late readiness changed a stopping service to Running")
		}
	}
}

func TestWindowsServiceReportsApplicationFailures(t *testing.T) {
	for _, running := range []bool{false, true} {
		name := "Startup"
		if running {
			name = "Runtime"
		}
		t.Run(name, func(t *testing.T) {
			failure := errors.New("application failed")
			fail := make(chan struct{})
			release := sync.OnceFunc(func() { close(fail) })
			t.Cleanup(release)
			h := startServiceHarness(t, func(ctx context.Context, ready func()) error {
				if running {
					ready()
				}
				select {
				case <-fail:
				case <-ctx.Done():
				}
				return failure
			})
			if running {
				h.expectState(t, svc.Running)
			}
			release()
			result := receiveWithin(t, h.done)
			if !result.specific || result.code == 0 || !errors.Is(h.handler.err, failure) {
				t.Fatalf("application error returned %+v, error %v", result, h.handler.err)
			}
			if !running && len(h.statuses) != 0 {
				t.Fatalf("startup failure emitted unexpected status: %+v", <-h.statuses)
			}
		})
	}
}
