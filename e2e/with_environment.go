// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package e2e

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"
	"os"
	"os/exec"
	"sync"
	"testing"
)

type WithTestCase struct {
	Name string
	Test func(t *testing.T, environment Environment, extension Extension)
}

func WithEnvironment(t *testing.T, environment Environment, extFactory ExtensionFactory, testCases []WithTestCase) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05.000"})
	ctx := t.Context()

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		err := extFactory.Create(ctx, environment)
		if err != nil {
			log.Fatal().Msgf("failed to create extension executable: %v", err)
		}
		wg.Done()
	}()

	wg.Wait()
	extension, err := extFactory.Start(ctx, environment)
	defer func() { _ = extFactory.Stop(ctx, environment, extension) }()
	require.NoError(t, err)

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Test(t, environment, extension)
		})
	}

	processCoverage(extension)
}

func processCoverage(extension Extension) {
	if _, err := extension.Client().R().SetOutput("covmeta.1").Get("/coverage/meta"); err != nil {
		log.Info().Err(err).Msg("failed to get coverage meta. Did you compile with `-cover`? Did you add the coverage endpoints ('action_kit_sdk.RegisterCoverageEndpoints()')?")
		return
	}
	if _, err := extension.Client().R().SetOutput("covcounters.1.1.1").Get("/coverage/counters"); err != nil {
		log.Info().Err(err).Msg("failed to get coverage meta. Did you compile with `-cover`? Did you add the coverage endpoints ('action_kit_sdk.RegisterCoverageEndpoints()')?")
		return
	}
	if err := exec.Command("go", "tool", "covdata", "textfmt", "-i", ".", "-o", "e2e-coverage.out").Run(); err != nil {
		log.Info().Err(err).Msg("failed to convert coverage data.")
		return
	}
	if err := os.Remove("covmeta.1"); err != nil {
		log.Info().Err(err).Msg("failed to clean up coverage meta data.")
		return
	}
	if err := os.Remove("covcounters.1.1.1"); err != nil {
		log.Info().Err(err).Msg("failed to clean up coverage counters data.")
		return
	}
}
