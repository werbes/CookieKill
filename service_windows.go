//go:build windows

package main

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

const serviceName = "CookieKill"

func runPlatform() error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("detect Windows service: %w", err)
	}
	if !isService {
		return runConsole()
	}
	handler := &windowsService{run: func(ctx context.Context, ready func()) error {
		err := runApplication(ctx, true, ready)
		if err != nil {
			reportServiceError(err)
		}
		return err
	}}
	if err := svc.Run(serviceName, handler); err != nil {
		reportServiceError(err)
		return err
	}
	return handler.err
}

type windowsService struct {
	run func(context.Context, func()) error
	err error
}

// Execute stays responsive while the application loads or saves progress.
// Checkpoints tell SCM that startup/shutdown is still in progress.
func (s *windowsService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	status := svc.Status{State: svc.StartPending, CheckPoint: 1, WaitHint: 15000}
	changes <- status
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- s.run(ctx, func() { close(ready) }) }()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ready:
			ready = nil
			if status.State == svc.StartPending {
				status = svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
				changes <- status
			}
		case err := <-done:
			s.err = err
			if err != nil {
				return true, 1
			}
			return false, 0
		case request, ok := <-requests:
			if !ok {
				requests = nil
				cancel()
				status = svc.Status{State: svc.StopPending, CheckPoint: 1, WaitHint: 15000}
				changes <- status
				continue
			}
			switch request.Cmd {
			case svc.Interrogate:
				changes <- status
			case svc.Stop, svc.Shutdown:
				if status.State != svc.StopPending {
					status = svc.Status{State: svc.StopPending, CheckPoint: 1, WaitHint: 15000}
					changes <- status
					cancel()
				}
			}
		case <-ticker.C:
			if status.State == svc.StartPending || status.State == svc.StopPending {
				status.CheckPoint++
				changes <- status
			}
		}
	}
}

func reportServiceError(err error) {
	if log, openErr := eventlog.Open(serviceName); openErr == nil {
		defer log.Close()
		_ = log.Error(1, "CookieKill stopped: "+err.Error())
	}
}
