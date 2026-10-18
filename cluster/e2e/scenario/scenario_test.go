package scenario

import (
	"testing"
	"time"

	"github.com/octelium/octelium/cluster/e2e/scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenarioSPIFFE(t *testing.T) {
	for _, id := range []string{DefaultScenario, CoreMainScenario} {
		t.Run(id, func(t *testing.T) {
			for _, val := range []string{"", "false", "true", "1"} {
				t.Run("SPIFFE="+val, func(t *testing.T) {
					t.Setenv(EnvSPIFFE, val)
					t.Setenv("OCTELIUM_SPIFFE_TRUST_DOMAIN", "")
					t.Setenv(EnvCoreVersion, "")
					s, err := scenario.Get(id)
					require.Nil(t, err)

					enabled := val == "true" || val == "1"
					assert.Equal(t, id, s.ID)
					assert.Equal(t, enabled, s.Install.EnableSPIFFECSI)
					assert.Equal(t, enabled, s.Caps.Has(scenario.CapSPIFFE))
					assert.Contains(t, s.Topology.Labels, NodeLabel)
					assert.Equal(t, 120*time.Minute, s.Budget)
					if id == CoreMainScenario {
						assert.Equal(t, "main", s.Install.Version)
					} else {
						assert.Empty(t, s.Install.Version)
					}

					if enabled {
						assert.Equal(t, scenario.SPIFFETrustDomainDefault, s.Install.SPIFFETrustDomain)
						require.Len(t, s.Hooks.PostPrepare, 1)
						assert.Equal(t, "spiffe/spire", s.Hooks.PostPrepare[0].Name)
						require.NotNil(t, s.Hooks.PostPrepare[0].Run)
					} else {
						assert.Empty(t, s.Install.SPIFFETrustDomain)
						assert.Empty(t, s.Hooks.PostPrepare)
					}
					require.Len(t, s.Hooks.PostInstall, 2)
					assert.Equal(t, "cordium/install-package", s.Hooks.PostInstall[0].Name)
					assert.Equal(t, "cordium/readiness", s.Hooks.PostInstall[1].Name)
				})
			}
		})
	}
}

func TestScenarioSPIFFETrustDomain(t *testing.T) {
	t.Setenv(EnvSPIFFE, "true")
	t.Setenv("OCTELIUM_SPIFFE_TRUST_DOMAIN", "cordium.test")

	for _, id := range []string{DefaultScenario, CoreMainScenario} {
		t.Run(id, func(t *testing.T) {
			s, err := scenario.Get(id)
			require.Nil(t, err)
			assert.Equal(t, "cordium.test", s.Install.SPIFFETrustDomain)
		})
	}
}
