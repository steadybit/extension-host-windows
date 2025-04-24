// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
)

var serviceName = "SteadybitExtensionHostWindows"
var applicationDataPath = filepath.Join(os.Getenv("ProgramData"), "Steadybit GmbH")
var logFilePath = filepath.Join(applicationDataPath, "extension-host-windows.log")

func ActivateWindowsServiceHandler(stopHandler func()) {
	isInService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to detect if executed as Windows service")
	}
	if isInService {
		go func() {
			service, err := newExtensionService(stopHandler)
			if err != nil {
				log.Fatal().Err(err).Msg("Error starting as Windows service")
				return
			}
			err = service.Run()
			if err != nil {
				log.Fatal().Err(err).Msg("Error starting as Windows service")
				return
			}
			log.Info().Msg("Windows service stopped")
		}()
	}
}

type extensionService struct {
	stopHandler func()
}

func newExtensionService(stopHandler func()) (*extensionService, error) {
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

	return &extensionService{
		stopHandler: func() {
			stopHandler()
			_ = elog.Close()
			_ = eventlog.Remove(serviceName)
		},
	}, nil
}

func (s *extensionService) Run() error {
	return svc.Run(serviceName, s)
}

func (s *extensionService) Execute(_ []string, changeRequests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	statuses <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	for {
		changeRequest := <-changeRequests
		switch changeRequest.Cmd {
		case svc.Stop, svc.Shutdown:
			s.stopHandler()
			statuses <- svc.Status{State: svc.Stopped, Accepts: cmdsAccepted}
			log.Fatal().Msg("Received Windows service stop command")
		default:
			log.Info().Msgf("Unexpected control request: cmd %d", changeRequest.Cmd)
		}
	}
}

type eventLogWriter struct {
	log *eventlog.Log
}

func (w *eventLogWriter) Write(p []byte) (n int, err error) {
	d := json.NewDecoder(bytes.NewReader(p))
	d.UseNumber()

	var event map[string]interface{}
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
