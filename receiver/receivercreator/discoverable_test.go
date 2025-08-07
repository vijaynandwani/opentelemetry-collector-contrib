// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	rcvr "go.opentelemetry.io/collector/receiver"
)

// mockDiscoverableReceiver implements both receiver.Factory and Discoverable for testing
type mockDiscoverableReceiver struct {
	validateFunc func(rawCfg map[string]any, discoveredEndpoint string) error
}

func (m *mockDiscoverableReceiver) Type() component.Type {
	return component.MustNewType("mock_discoverable")
}

func (m *mockDiscoverableReceiver) CreateDefaultConfig() component.Config {
	return &mockDiscoverableConfig{}
}

func (m *mockDiscoverableReceiver) CreateLogsReceiver(
	ctx component.Config,
	set rcvr.Settings,
	nextConsumer component.Component,
) (rcvr.Logs, error) {
	return nil, nil
}

func (m *mockDiscoverableReceiver) CreateMetricsReceiver(
	ctx component.Config,
	set rcvr.Settings,
	nextConsumer component.Component,
) (rcvr.Metrics, error) {
	return nil, nil
}

func (m *mockDiscoverableReceiver) CreateTracesReceiver(
	ctx component.Config,
	set rcvr.Settings,
	nextConsumer component.Component,
) (rcvr.Traces, error) {
	return nil, nil
}

// mockDiscoverableConfig implements both component.Config and Discoverable
type mockDiscoverableConfig struct {
	validateFunc func(rawCfg map[string]any, discoveredEndpoint string) error
}

func (m *mockDiscoverableConfig) Validate(rawCfg map[string]any, discoveredEndpoint string) error {
	if m.validateFunc != nil {
		return m.validateFunc(rawCfg, discoveredEndpoint)
	}
	return nil
}

// mockNonDiscoverableConfig is a regular config that doesn't implement Discoverable
type mockNonDiscoverableConfig struct{}

func TestMergeTemplatedAndDiscoveredConfigs_WithDiscoverableReceiver(t *testing.T) {
	tests := []struct {
		name             string
		templated        userConfigMap
		discovered       userConfigMap
		validateFunc     func(rawCfg map[string]any, discoveredEndpoint string) error
		expectedEndpoint string
		expectError      bool
	}{
		{
			name: "successful discoverable validation",
			templated: userConfigMap{
				"job_name": "test-job",
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "discovered-app",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"`endpoint`"},
								},
							},
						},
					},
				},
			},
			discovered: userConfigMap{
				endpointConfigKey:          "10.1.2.3:8080",
				tmpSetEndpointConfigKey:    struct{}{},
			},
			validateFunc: func(rawCfg map[string]any, discoveredEndpoint string) error {
				// Mock successful validation
				return nil
			},
			expectedEndpoint: "10.1.2.3:8080",
			expectError:      false,
		},
		{
			name: "failed discoverable validation",
			templated: userConfigMap{
				"config": map[string]any{
					"scrape_configs": []any{
						map[string]any{
							"job_name": "malicious-job",
							"static_configs": []any{
								map[string]any{
									"targets": []any{"evil.com:9090"},
								},
							},
						},
					},
				},
			},
			discovered: userConfigMap{
				endpointConfigKey:          "10.1.2.3:8080",
				tmpSetEndpointConfigKey:    struct{}{},
			},
			validateFunc: func(rawCfg map[string]any, discoveredEndpoint string) error {
				return assert.AnError // Mock validation failure
			},
			expectedEndpoint: "10.1.2.3:8080",
			expectError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock factory that returns discoverable config
			factory := &mockDiscoverableReceiver{}
			factory.CreateDefaultConfig = func() component.Config {
				return &mockDiscoverableConfig{
					validateFunc: tt.validateFunc,
				}
			}

			result, endpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, tt.templated, tt.discovered)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "discoverable validation failed")
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				
				// Verify endpoint was not injected for discoverable receivers
				resultMap := result.ToStringMap()
				_, hasEndpoint := resultMap[endpointConfigKey]
				assert.False(t, hasEndpoint, "Discoverable receivers should not have endpoint field injected")
			}

			assert.Equal(t, tt.expectedEndpoint, endpoint)
		})
	}
}

func TestMergeTemplatedAndDiscoveredConfigs_WithNonDiscoverableReceiver(t *testing.T) {
	// Create mock factory that returns non-discoverable config
	factory := &mockDiscoverableReceiver{}
	factory.CreateDefaultConfig = func() component.Config {
		return &mockNonDiscoverableConfig{}
	}

	templated := userConfigMap{
		"collection_interval": "30s",
	}
	discovered := userConfigMap{
		endpointConfigKey:       "10.1.2.3:8080",
		tmpSetEndpointConfigKey: struct{}{},
	}

	result, endpoint, err := mergeTemplatedAndDiscoveredConfigs(factory, templated, discovered)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "10.1.2.3:8080", endpoint)

	// Verify the old behavior still works for non-discoverable receivers
	// (endpoint injection logic should still run)
	resultMap := result.ToStringMap()
	assert.Equal(t, "30s", resultMap["collection_interval"])
}
