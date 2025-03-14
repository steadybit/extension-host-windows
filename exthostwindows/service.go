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
	"time"
)

var serviceName = "SteadybitExtensionHostWindows"

type ExtensionService struct {
	stopHandler func()
}

func (s *ExtensionService) Execute(_ []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}
	tick := time.Tick(500 * time.Millisecond)
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	for {
		select {
		case <-tick:
			log.Trace().Msg("tick")
		case c := <-r:
			switch c.Cmd {
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.Stopped, Accepts: cmdsAccepted}
				log.Fatal().Msg("Received Windows service stop command")
			default:
				log.Info().Msgf("unexpected control request #%d", c)
			}
		}
	}
}

func NewExtensionService(stopHandler func()) error {
	elog, err := eventlog.Open(serviceName)
	if err != nil {
		return err
	}
	defer elog.Close()

	logDir := filepath.Join(os.Getenv("ProgramData"), serviceName, "logs")
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
	} else {
		_ = elog.Info(1, fmt.Sprintf("Log file opened: %s", logPath))
	}

	defer logFile.Close()

	elw := &eventLogWriter{
		log: elog,
	}
	log.Logger = log.Output(elw)

	extensionService := &ExtensionService{
		stopHandler: stopHandler,
	}
	return svc.Run("SteadybitExtensionHostWindows", extensionService)
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
