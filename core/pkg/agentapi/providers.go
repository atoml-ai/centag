// Package agentapi exposes Agent provider helpers for commercial plugins.
package agentapi

import (
	"fmt"

	"centag/core/internal/agent"
	"centag/core/pkg/logger"
)

// CopySystemProvidersToTenant copies system-default agent provider configs
// (empty TenantID) into the given tenant, clearing API keys.
func CopySystemProvidersToTenant(tenantID string) (copied int, err error) {
	if tenantID == "" {
		return 0, fmt.Errorf("tenant_id is required")
	}
	mgr := agent.GetProviderManager()
	systemConfigs := mgr.List()

	for _, cfg := range systemConfigs {
		if cfg.TenantID != "" {
			continue
		}
		newCfg := *cfg
		newCfg.ID = fmt.Sprintf("%s_%s", tenantID, cfg.ID)
		newCfg.TenantID = tenantID
		newCfg.APIKey = ""

		if err := mgr.Update(&newCfg); err != nil {
			if err := mgr.Add(&newCfg); err != nil {
				logger.Warnf("Failed to copy agent provider %s for tenant %s: %v", cfg.ID, tenantID, err)
				continue
			}
		}
		copied++
	}

	if err := mgr.Save(); err != nil {
		return copied, fmt.Errorf("failed to save copied agent providers: %w", err)
	}

	logger.Info("System agent providers copied to tenant",
		logger.GetField("tenant_id", tenantID),
		logger.GetField("copied_count", copied))
	return copied, nil
}
