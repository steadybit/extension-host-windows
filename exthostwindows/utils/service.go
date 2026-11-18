// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package utils

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const servicePollInterval = 50 * time.Millisecond

// StopService stops the given Windows service and waits until it is stopped. A stopped service is left as is.
// A service that does not exist yields windows.ERROR_SERVICE_DOES_NOT_EXIST.
func StopService(name string, timeout time.Duration) error {
	return withService(name, func(s *mgr.Service) error {
		return awaitServiceState(context.Background(), s, svc.Stopped, timeout, func(state svc.State) {
			if state == svc.Running || state == svc.Paused {
				if _, err := s.Control(svc.Stop); err != nil {
					log.Debug().Err(err).Str("service", name).Msg("failed to stop service, retrying")
				}
			}
		})
	})
}

// StartService starts the given Windows service and waits until it is running. A running service is left as is.
func StartService(name string, timeout time.Duration) error {
	return withService(name, func(s *mgr.Service) error {
		return awaitServiceState(context.Background(), s, svc.Running, timeout, func(state svc.State) {
			if state == svc.Stopped {
				if err := s.Start(); err != nil {
					log.Debug().Err(err).Str("service", name).Msg("failed to start service, retrying")
				}
			}
		})
	})
}

// AwaitServiceRunning waits until the given service exists and is running, e.g. a driver service installed on demand.
// It stops waiting when ctx ends, e.g. because the process that installs the service exited.
func AwaitServiceRunning(ctx context.Context, name string, timeout time.Duration) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()

	end := time.Now().Add(timeout)
	for {
		s, err := m.OpenService(name)
		if err == nil {
			defer func() { _ = s.Close() }()
			return awaitServiceState(ctx, s, svc.Running, time.Until(end), nil)
		}
		if !errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) || time.Now().After(end) {
			return err
		}
		if err := pollWait(ctx); err != nil {
			return err
		}
	}
}

func withService(name string, fn func(s *mgr.Service) error) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() { _ = m.Disconnect() }()

	s, err := m.OpenService(name)
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	return fn(s)
}

// awaitServiceState polls until the service reaches the wanted state. onState, if set, can act on every other state.
func awaitServiceState(ctx context.Context, s *mgr.Service, wanted svc.State, timeout time.Duration, onState func(svc.State)) error {
	end := time.Now().Add(timeout)
	for time.Now().Before(end) {
		status, err := s.Query()
		if err == nil {
			if status.State == wanted {
				log.Debug().Str("service", s.Name).Int("state", int(wanted)).Msg("service reached state")
				return nil
			}
			if onState != nil {
				onState(status.State)
			}
		}
		if err := pollWait(ctx); err != nil {
			return err
		}
	}
	return fmt.Errorf("service %s did not reach state %d within %s", s.Name, wanted, timeout)
}

func pollWait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	case <-time.After(servicePollInterval):
		return nil
	}
}
