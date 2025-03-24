// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type LocalExtensionFactory struct {
	Name       string
	Port       int
	Executable string
	Command    *exec.Cmd
	ExtraArgs  func() []string
	ExtraEnv   func() map[string]string
}

func (f *LocalExtensionFactory) Create(ctx context.Context, e Environment) error {
	start := time.Now()
	err := e.StartProcess(ctx, "make", "artifact")
	if err != nil {
		return err
	}

	artifact, err := findExtensionArtifact(distPath)
	if err != nil {
		return err
	}
	log.Info().
		TimeDiff("duration", time.Now(), start).
		Str("artifact", artifact).
		Msg("extension created")

	artifactDir, err := extractArtifact(artifact)
	if err != nil {
		return err
	}
	log.Info().
		TimeDiff("duration", time.Now(), start).
		Str("artifactDir", artifactDir).
		Msg("extension extracted")

	artifactExecutable, err := findExtensionExecutable(artifactDir)
	if err != nil {
		return err
	}
	log.Info().
		TimeDiff("duration", time.Now(), start).
		Str("artifactExecutable", artifactExecutable).
		Msg("extension executable found")
	f.Executable = artifactExecutable

	return nil
}

func (f *LocalExtensionFactory) Start(ctx context.Context, _ Environment) (Extension, error) {
	err := f.startAndAwait(ctx)
	if err != nil {
		return nil, err
	}
	//e2eTest, err := os.Executable()
	//if err != nil {
	//	return nil, err
	//}
	//err = f.addFirewallRules(ctx, e2eTest)
	//if err != nil {
	//	return nil, err
	//}
	//err = f.addFirewallRules(ctx, f.Executable)
	//if err != nil {
	//	return nil, err
	//}

	ext := NewLocalExtension(f.Port)
	return ext, err
}

func (f *LocalExtensionFactory) Stop(ctx context.Context, _ Environment, _ Extension) error {
	if f.Command != nil {
		err := f.Command.Process.Kill()
		if err != nil {
			log.Error().Err(err).Msg("failed to kill")
		}
	}
	return nil
	//e2eTest, err := os.Executable()
	//if err != nil {
	//	return err
	//}
	//err = f.removeFirewallRules(ctx, e2eTest)
	//if err != nil {
	//	return err
	//}
	//return f.removeFirewallRules(ctx, f.Executable)
}

func (f *LocalExtensionFactory) startAndAwait(ctx context.Context) error {
	log.Info().Msg("starting extension")
	var args []string
	if f.ExtraArgs != nil {
		args = f.ExtraArgs()
	}
	cmd := exec.CommandContext(ctx, f.Executable, args...)
	cmd.Stdout = &PrefixWriter{prefix: []byte("🧊 "), w: os.Stdout}
	cmd.Stderr = &PrefixWriter{prefix: []byte("🧊 "), w: os.Stderr}

	currentEnv := os.Environ()
	customEnv := append(currentEnv, fmt.Sprintf("STEADYBIT_EXTENSION_PORT=%d", f.Port))
	for k, v := range f.ExtraEnv() {
		customEnv = append(customEnv, fmt.Sprintf("%s=%s", k, v))
	}
	var pathEnv string
	for _, envVar := range customEnv {
		if strings.HasPrefix(envVar, "PATH=") {
			pathEnv = envVar
		}
	}
	if pathEnv == "" {
		pathEnv = "PATH=" + filepath.Dir(f.Executable)
	} else {
		pathEnv = pathEnv + ";" + filepath.Dir(f.Executable)
	}

	log.Info().Str("path", pathEnv).Msg("Setting custom path environment")

	customEnv = append(customEnv, pathEnv)
	cmd.Env = customEnv

	err := awaitStartup(cmd, awaitLog("Starting extension http server on port"))
	if err != nil {
		return err
	}
	log.Info().Strs("cmd", cmd.Args).Msg("started extension")
	f.Command = cmd
	return nil
}

const ruleNameFormat = "Steadybit Extension Host Windows e2e %s"

func (f *LocalExtensionFactory) addFirewallRules(ctx context.Context, executable string) error {
	ruleName := fmt.Sprintf(ruleNameFormat, filepath.Base(executable))
	log.Info().Msgf("adding firewall rules - %s", ruleName)

	psCommand := fmt.Sprintf(
		"New-NetFirewallRule -DisplayName '%s' -Direction Inbound -Program '%s' -Action Allow -Profile Public,Private,Domain -Protocol TCP -LocalPort Any; "+
			"New-NetFirewallRule -DisplayName '%s - Outbound' -Direction Outbound -Program '%s' -Action Allow -Profile Public,Private,Domain -Protocol TCP -RemotePort Any",
		executable, ruleName, executable, ruleName)
	cmd := exec.CommandContext(ctx, "powershell", "-Command", psCommand)

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error creating firewall rule: %v", err)
	}

	// Give time for the rule to be applied
	time.Sleep(1 * time.Second)
	return nil
}

func (f *LocalExtensionFactory) removeFirewallRules(ctx context.Context, executable string) error {
	ruleName := fmt.Sprintf(ruleNameFormat, filepath.Base(executable))
	log.Info().Msgf("removing firewall rules - %s", ruleName)

	psCommand := fmt.Sprintf(
		"Remove-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue; "+
			"Remove-NetFirewallRule -DisplayName '%s' -ErrorAction SilentlyContinue", ruleName, ruleName)
	cmd := exec.CommandContext(ctx, "powershell", "-Command", psCommand)

	return cmd.Run()
}
