// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2025 Steadybit GmbH

package exthostwindows

import (
	"context"
	"github.com/steadybit/extension-host-windows/config"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_DiscoverTargets(t *testing.T) {
	_ = os.Setenv("steadybit_label_Foo", "Bar")
	_ = os.Setenv("STEADYBIT_DISCOVERY_ENV_LIST", "MyEnvVar,MyEnvVar2;MyEnvVar3")
	_ = os.Setenv("MyEnvVar", "MyEnvVarValue")
	_ = os.Setenv("MyEnvVar2", "MyEnvVarValue2")
	_ = os.Setenv("MyEnvVar3", "MyEnvVarValue3")
	config.Config.DiscoveryAttributesExcludesHost = []string{hostNicAttribute}

	targets, _ := (&hostDiscovery{}).DiscoverTargets(context.Background())

	assert.NotNil(t, targets)
	assert.Len(t, targets, 1)
	target := targets[0]
	assert.NotEmpty(t, target.Id)
	assert.NotEmpty(t, target.Label)
	assert.NotEmpty(t, target.Attributes)
	attributes := target.Attributes
	assert.NotEmpty(t, attributes[hostNameAttribute])
	assert.NotEmpty(t, attributes[hostDomainnameAttribute])
	assert.NotEmpty(t, attributes[hostIp4Attribute])
	assert.NotContains(t, attributes, hostNicAttribute)
	assert.NotEmpty(t, attributes[hostOsFamilyAttribute])
	assert.NotEmpty(t, attributes[hostOsManufacturerAttribute])
	assert.NotEmpty(t, attributes[hostOsVersionAttribute])
	assert.Equal(t, attributes[hostLabelAttributePrefix+"foo"], []string{"Bar"})
	assert.Equal(t, attributes[hostEnvAttributePrefix+"myenvvar"], []string{"MyEnvVarValue"})
	assert.Equal(t, attributes[hostEnvAttributePrefix+"myenvvar2"], []string{"MyEnvVarValue2"})
	assert.Equal(t, attributes[hostEnvAttributePrefix+"myenvvar3"], []string{"MyEnvVarValue3"})
}
