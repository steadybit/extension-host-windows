// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/extension-kit/exthttp"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"gopkg.in/natefinch/lumberjack.v2"
)

var serviceName = "SteadybitExtensionHostWindows"
var applicationDataPath = filepath.Join(os.Getenv("ProgramData"), "Steadybit GmbH")
var logFilePath = filepath.Join(applicationDataPath, "extension-host-windows.log")

// waitHint is in milliseconds. Startup includes WMI queries; stopping rolls back
// running actions.
const waitHint = 30_000

// fatalExitCode is reported when the extension fails with log.Fatal. The SCM logs it
// as "An internal error occurred."; a service-specific code would be rendered with
// an unrelated system message (1 reads "Incorrect function.").
const fatalExitCode = uint32(windows.ERROR_INTERNAL_ERROR)

// ActivateWindowsServiceHandler exits the process only after the service control
// manager has been told that the service stopped, also on a later log.Fatal. Exiting any
// earlier, e.g. by returning from main, makes the SCM report the service as
// "terminated unexpectedly" (error 1067). The service reports RUNNING once the
// extension serves HTTP. It returns whether the process runs as a service.
func ActivateWindowsServiceHandler(stopHandler func()) (isService bool) {
	isInService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to detect if executed as Windows service")
	}
	if !isInService {
		return false
	}

	// Attach the file/event-log writer synchronously, before returning to the
	// startup sequence that emits the first log lines (e.g. build information).
	// Only the blocking svc.Run loop runs in the goroutine; doing the writer
	// setup there raced with startup logging and dropped the earliest lines.
	service, err := newExtensionService(stopHandler)
	if err != nil {
		log.Fatal().Err(err).Msg("Error starting as Windows service")
	}

	// Go turns the console CTRL_SHUTDOWN_EVENT into SIGTERM. While SIGTERM is
	// notified, Go's console handler blocks instead of letting Windows end the
	// process, so the SCM's shutdown control can run the stop handler.
	signal.Notify(make(chan os.Signal, 1), syscall.SIGTERM)

	go func() {
		exthttp.WaitForServe()
		close(service.ready)
	}()

	zerolog.FatalExitFunc = func() {
		service.fail()
		// The goroutine below exits once SERVICE_STOPPED is reported. That never
		// happens if log.Fatal was called on the Execute goroutine itself.
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}

	go func() {
		if err := service.Run(); err != nil {
			log.Error().Err(err).Msg("Error running as Windows service")
			os.Exit(1)
		}
		select {
		case <-service.failed:
			os.Exit(1)
		default:
			os.Exit(0)
		}
	}()
	return true
}

type extensionService struct {
	stopHandler func()
	ready       chan struct{}
	failed      chan struct{}
	fail        func()
}

func newExtensionService(stopHandler func()) (*extensionService, error) {
	eventlog.InstallAsEventCreate("SteadybitExtensionHostWindows", eventlog.Info|eventlog.Error|eventlog.Warning)
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		return nil, err
	}
	elw := &eventLogWriter{
		log: elog,
	}

	if err = os.MkdirAll(applicationDataPath, 0755); err != nil {
		_ = elog.Error(1, fmt.Sprintf("Failed to create log directory: %v", err))
	}

	logFile, err := os.OpenFile(
		logFilePath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0664,
	)
	if err != nil {
		_ = elog.Error(1, fmt.Sprintf("Failed to open log file: %v", err))
		return nil, err
	}
	_ = logFile.Close()
	_ = elog.Info(1, fmt.Sprintf("Log file opened: %s", logFilePath))

	flw := &lumberjack.Logger{
		Filename:   logFilePath,
		MaxBackups: 10,
		MaxSize:    5,
	}

	log.Logger = log.Output(io.MultiWriter(flw, elw))

	failed := make(chan struct{})

	return &extensionService{
		stopHandler: func() {
			stopHandler()
			_ = elog.Close()
			_ = eventlog.Remove(serviceName)
		},
		ready:  make(chan struct{}),
		failed: failed,
		fail:   sync.OnceFunc(func() { close(failed) }),
	}, nil
}

func (s *extensionService) Run() error {
	return svc.Run(serviceName, s)
}

func (s *extensionService) Execute(_ []string, changeRequests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending, WaitHint: waitHint}
	ready := s.ready
	for {
		select {
		case <-ready:
			ready = nil
			statuses <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}
		case <-s.failed:
			// svc.Run reports SERVICE_STOPPED with this exit code once Execute has returned.
			return false, fatalExitCode
		case changeRequest := <-changeRequests:
			switch changeRequest.Cmd {
			case svc.Interrogate:
				statuses <- changeRequest.CurrentStatus
			case svc.Stop, svc.Shutdown:
				statuses <- svc.Status{State: svc.StopPending, WaitHint: waitHint}
				log.Info().Msg("Received Windows service stop command")
				s.stopHandler()
				return false, 0
			default:
				log.Info().Msgf("Unexpected control request: cmd %d", changeRequest.Cmd)
			}
		}
	}
}

type eventLogWriter struct {
	log *eventlog.Log
}

func (w *eventLogWriter) Write(p []byte) (n int, err error) {
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()

	var event map[string]any
	err = d.Decode(&event)
	if err != nil {
		return 0, err
	}

	var logFn = w.log.Info
	if l, ok := event[zerolog.LevelFieldName].(string); ok {
		logFn = w.mapLevel(l)
	}
	if m, ok := event[zerolog.MessageFieldName].(string); ok {
		return 0, logFn(1, m)
	}
	// If no message field is present log everything
	return 0, logFn(1, string(p))
}

func (w *eventLogWriter) mapLevel(zLevel string) func(uint32, string) error {
	lvl, _ := zerolog.ParseLevel(zLevel)
	switch lvl {
	case zerolog.NoLevel, zerolog.Disabled:
		return func(uint32, string) error {
			return nil
		}
	case zerolog.TraceLevel, zerolog.DebugLevel, zerolog.InfoLevel:
		return w.log.Info
	case zerolog.WarnLevel:
		return w.log.Warning
	case zerolog.ErrorLevel, zerolog.FatalLevel, zerolog.PanicLevel:
		return w.log.Error
	}
	return w.log.Info
}
