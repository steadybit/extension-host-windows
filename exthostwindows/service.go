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
	"os"
	"path/filepath"
)

var serviceName = "SteadybitExtensionHostWindows"
var logDir = filepath.Join(os.Getenv("ProgramData"), "Steadybit GmbH", "Windows Host Extension", "Core")

type ExtensionService struct {
	stopHandler func()
}

func ActivateWindowsServiceHandler(stopHandler func()) {
	isInService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to detect if executed as Windows service")
	}
	if isInService {
		go func() {
			service, err := NewExtensionService(stopHandler)
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

func NewExtensionService(stopHandler func()) (*ExtensionService, error) {
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		return nil, err
	}
	defer func(elog *eventlog.Log) {
		_ = elog.Close()
	}(elog)

	if err = os.MkdirAll(logDir, 0755); err != nil {
		_ = elog.Error(1, fmt.Sprintf("Failed to create log directory: %v", err))
	}

	logPath := filepath.Join(logDir, "extension.log")
	logFile, err := os.OpenFile(
		logPath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0664,
	)
	if err != nil {
		_ = elog.Error(1, fmt.Sprintf("Failed to open log file: %v", err))
		return nil, err
	}
	_ = elog.Info(1, fmt.Sprintf("Log file opened: %s", logPath))

	defer func(logFile *os.File) {
		_ = logFile.Close()
	}(logFile)

	elw := &eventLogWriter{
		log: elog,
	}
	log.Logger = log.Output(elw)

	return &ExtensionService{
		stopHandler: stopHandler,
	}, nil
}

func (s *ExtensionService) Run() error {
	return svc.Run(serviceName, s)
}

func (s *ExtensionService) Execute(_ []string, changeRequests <-chan svc.ChangeRequest, statuses chan<- svc.Status) (ssec bool, errno uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	statuses <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	for {
		changeRequest := <-changeRequests
		switch changeRequest.Cmd {
		case svc.Stop, svc.Shutdown:
			statuses <- svc.Status{State: svc.Stopped, Accepts: cmdsAccepted}
			log.Fatal().Msg("Received Windows service stop command")
			s.stopHandler()
		default:
			log.Info().Msgf("unexpected control request #%d", changeRequest)
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
		return
	}

	var logFn = w.log.Info
	if l, ok := event[zerolog.LevelFieldName].(string); ok {
		logFn = w.mapLevel(l)
	}
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
