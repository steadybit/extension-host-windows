// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package config

import (
	"github.com/kelseyhightower/envconfig"
	"github.com/rs/zerolog/log"
)

type Specification struct {
	Port                            uint16   `json:"port" split_words:"true" required:"false" default:"8085"`
	HealthPort                      uint16   `json:"healthPort" split_words:"true" required:"false" default:"8081"`
	DiscoveryAttributesExcludesHost []string `json:"discoveryAttributesExcludesHost" split_words:"true" required:"false"`
	StartAsService                  bool     `json:"startAsService" split_words:"true" default:"false"`
}

var (
	Config Specification
)

func ParseConfiguration() {
	err := envconfig.Process("steadybit_extension", &Config)
	if err != nil {
		log.Fatal().Err(err).Msgf("Failed to parse configuration from environment.")
	}
}

func ValidateConfiguration() {
	// You may optionally validate the configuration here.
}
