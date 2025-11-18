package tenantconfig

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// DefaultFileName is the default path used to load the tenant config when
// LoadDefault is called.
const DefaultFileName = "./tenantConfig.yml"

// EnvVarPath allows overriding the default tenant config file path.
// If set and non-empty, LoadDefault will attempt to load from this path.
const EnvVarPath = "TENANT_CONFIG_PATH"

// TenantConfig represents the configuration for a tenant as defined in the YAML file.
//
// Example YAML:
// tenants:
//
//	tenantA:
//	  dbUrl: postgres://...
//	  config:
//	    any: value
//
// Note: "dbUrl" is required for each tenant.
// Additional keys under "config" are arbitrary and left to the user.
// If you need strict typing for the nested config, replace map[string]any with a struct.
//
// The fields are exported to support consumers outside this package.
// Use GetTenantConfig to access individual tenant configs safely.
//
//nolint:revive // keep exported names to be used by routing logic elsewhere
type TenantConfig struct {
	DBURL  string         `yaml:"dbUrl" json:"dbUrl"`
	Config map[string]any `yaml:"config" json:"config"`
}

// fileRoot mirrors the YAML root structure.
type fileRoot struct {
	Tenants map[string]TenantConfig `yaml:"tenants"`
}

var (
	mu              sync.RWMutex
	TenantConfigMap map[string]TenantConfig
)

// LoadDefault loads the tenant configuration on startup using either the path
// specified by TENANT_CONFIG_PATH environment variable or the default
// ./tenantConfig.yml file if the env var is unset.
// It parses YAML, validates required fields, and stores results in memory.
//
// Returns an error if the file cannot be read or parsed, or if required fields
// are missing. The map is updated atomically on successful load.
func LoadDefault() error {
	path := os.Getenv(EnvVarPath)
	if path == "" {
		path = DefaultFileName
	}
	return LoadFromFile(path)
}

// LoadFromFile reads the given YAML file, parses it, validates content, and
// stores the result in a global in-memory map for runtime lookups.
func LoadFromFile(path string) error {
	// Resolve for nicer error messages.
	abs := path
	if a, err := filepath.Abs(path); err == nil {
		abs = a
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("tenant config file not found at %s: %w", abs, err)
		}
		return fmt.Errorf("failed reading tenant config file %s: %w", abs, err)
	}

	var root fileRoot
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("invalid YAML in tenant config %s: %w", abs, err)
	}

	// Basic structure validation
	if len(root.Tenants) == 0 {
		return fmt.Errorf("tenant config %s must define 'tenants' with at least one entry", abs)
	}

	// Validate each tenant
	validated := make(map[string]TenantConfig, len(root.Tenants))
	for id, cfg := range root.Tenants {
		if id == "" {
			return fmt.Errorf("tenant with empty id is not allowed in %s", abs)
		}
		if cfg.DBURL == "" {
			return fmt.Errorf("tenant '%s' is missing required field 'dbUrl' in %s", id, abs)
		}
		if cfg.Config == nil {
			cfg.Config = make(map[string]any)
		}
		validated[id] = cfg
	}

	// Atomically swap the map under write lock
	mu.Lock()
	TenantConfigMap = validated
	mu.Unlock()

	return nil
}

// GetTenantConfig returns a copy of the configuration for the provided tenant id.
// Returns (nil, error) if the tenant does not exist or if the configuration has
// not been loaded yet.
func GetTenantConfig(tenantID string) (*TenantConfig, error) {
	mu.RLock()
	defer mu.RUnlock()

	if TenantConfigMap == nil {
		return nil, errors.New("tenant configuration not loaded")
	}
	cfg, ok := TenantConfigMap[tenantID]
	if !ok {
		return nil, fmt.Errorf("tenant '%s' not found", tenantID)
	}
	// Return a copy to avoid external mutation of internal state
	copyCfg := cfg
	if cfg.Config != nil {
		copyMap := make(map[string]any, len(cfg.Config))
		for k, v := range cfg.Config {
			copyMap[k] = v
		}
		copyCfg.Config = copyMap
	}
	return &copyCfg, nil
}

// AllTenants returns a shallow copy of the internal tenant map for read-only
// scenarios where the full set is needed. Callers should treat the returned
// map as immutable.
func AllTenants() map[string]TenantConfig {
	mu.RLock()
	defer mu.RUnlock()

	out := make(map[string]TenantConfig, len(TenantConfigMap))
	for k, v := range TenantConfigMap {
		out[k] = v
	}
	return out
}
