// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2026 Steadybit GmbH

package network

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/rs/zerolog/log"
)

const (
	// wmiCallTimeout bounds a single WMI call. WMI providers can stop responding, and a stuck call must not block
	// every later QoS operation.
	wmiCallTimeout  = 30 * time.Second
	qosWmiNamespace = `root\StandardCimv2`
	// sFalse is returned by CoInitializeEx when the thread has COM initialized already, which is no error.
	sFalse = 1
)

// errWmiTimeout marks a call that was abandoned. It may still be running, so the caller must not read what fn writes.
var errWmiTimeout = errors.New("WMI call timed out")

// wmiConnection runs all calls on one OS thread that initializes COM once and reuses the connection to the
// namespace. Connecting per call would repeat the COM and WMI client setup, which costs time and, on some Windows
// versions, leaks memory (https://github.com/go-ole/go-ole/issues/135).
type wmiConnection struct {
	calls chan wmiCall
	quit  chan struct{}
}

type wmiCall struct {
	fn   func(svc *ole.IDispatch) error
	done chan error
}

var (
	qosWmiLock sync.Mutex
	qosWmi     *wmiConnection
)

func withQosWmi(ctx context.Context, fn func(svc *ole.IDispatch) error) error {
	conn := currentQosWmi()
	call := wmiCall{fn: fn, done: make(chan error, 1)}
	timeout := time.NewTimer(wmiCallTimeout)
	defer timeout.Stop()

	// While waiting for a running call, the connection is not discarded: the running call has its own timeout.
	select {
	case conn.calls <- call:
	case <-conn.quit:
		return errors.New("WMI connection was closed")
	case <-ctx.Done():
		return ctx.Err()
	case <-timeout.C:
		return fmt.Errorf("%w: no free WMI connection within %s", errWmiTimeout, wmiCallTimeout)
	}

	// A running call is only abandoned when it gets stuck, not when ctx ends, as fn writes into its caller's variables.
	timeout.Reset(wmiCallTimeout)
	select {
	case err := <-call.done:
		return err
	case <-timeout.C:
		discardQosWmi(conn)
		return fmt.Errorf("%w: no result within %s", errWmiTimeout, wmiCallTimeout)
	}
}

func currentQosWmi() *wmiConnection {
	qosWmiLock.Lock()
	defer qosWmiLock.Unlock()
	if qosWmi == nil {
		qosWmi = &wmiConnection{calls: make(chan wmiCall), quit: make(chan struct{})}
		go qosWmi.run()
	}
	return qosWmi
}

// discardQosWmi replaces a connection that got stuck or could not initialize COM; the next call starts a new one.
func discardQosWmi(conn *wmiConnection) {
	qosWmiLock.Lock()
	defer qosWmiLock.Unlock()
	if qosWmi == conn {
		qosWmi = nil
		close(conn.quit)
		log.Warn().Msg("discarded the WMI connection")
	}
}

func (c *wmiConnection) run() {
	// COM objects must be used on the thread that created them. The thread is never unlocked, so it terminates
	// together with this goroutine.
	runtime.LockOSThread()

	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != sFalse {
			log.Error().Err(err).Msg("failed to initialize COM for WMI")
			discardQosWmi(c)
			return
		}
	}
	defer ole.CoUninitialize()

	var svc *ole.IDispatch
	defer func() {
		if svc != nil {
			svc.Release()
		}
	}()

	for {
		select {
		case <-c.quit:
			return
		case call := <-c.calls:
			if svc == nil {
				var err error
				if svc, err = connectWmi(qosWmiNamespace); err != nil {
					call.done <- err
					continue
				}
			}
			err := call.fn(svc)
			if err != nil {
				// The error may stem from a broken connection; reconnecting costs only a few milliseconds.
				svc.Release()
				svc = nil
			}
			call.done <- err
		}
	}
}

func connectWmi(namespace string) (*ole.IDispatch, error) {
	unknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, fmt.Errorf("failed to create the WMI locator: %w", err)
	}
	defer unknown.Release()

	locator, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("failed to create the WMI locator: %w", err)
	}
	defer locator.Release()

	svc, err := oleutil.CallMethod(locator, "ConnectServer", ".", namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WMI namespace %s: %w", namespace, err)
	}
	return svc.ToIDispatch(), nil
}
