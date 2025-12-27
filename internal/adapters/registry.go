package adapters

import (
	"fmt"
	"strings"
	"sync"
)

// Registry manages all registered database adapters
type Registry struct {
	adapters    map[string]DatabaseAdapter
	aliasToName map[string]string
	mu          sync.RWMutex
}

var (
	// defaultRegistry is the global registry instance
	defaultRegistry *Registry
	once            sync.Once
)

// GetRegistry returns the global registry instance
func GetRegistry() *Registry {
	once.Do(func() {
		defaultRegistry = &Registry{
			adapters:    make(map[string]DatabaseAdapter),
			aliasToName: make(map[string]string),
		}
		// Register default adapters
		defaultRegistry.Register(NewPostgresAdapter())
		defaultRegistry.Register(NewMySQLAdapter())
		defaultRegistry.Register(NewRedisAdapter())
	})
	return defaultRegistry
}

// Register registers a new database adapter
func (r *Registry) Register(adapter DatabaseAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := adapter.GetName()
	r.adapters[name] = adapter

	// Register all aliases
	for _, alias := range adapter.GetAliases() {
		r.aliasToName[strings.ToLower(alias)] = name
	}
}

// Get retrieves an adapter by name or alias
func (r *Registry) Get(nameOrAlias string) (DatabaseAdapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSpace(nameOrAlias))

	// Try direct lookup
	if adapter, ok := r.adapters[normalized]; ok {
		return adapter, nil
	}

	// Try alias lookup
	if canonicalName, ok := r.aliasToName[normalized]; ok {
		if adapter, ok := r.adapters[canonicalName]; ok {
			return adapter, nil
		}
	}

	return nil, fmt.Errorf("unknown database type: %s", nameOrAlias)
}

// List returns all registered adapter names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	return names
}

// IsValidType checks if a database type is valid
func (r *Registry) IsValidType(dbType string) bool {
	_, err := r.Get(dbType)
	return err == nil
}

// NormalizeType normalizes a database type to its canonical name
func (r *Registry) NormalizeType(dbType string) (string, error) {
	adapter, err := r.Get(dbType)
	if err != nil {
		return "", err
	}
	return adapter.GetName(), nil
}

// GetAllAliases returns a map of all aliases to their canonical names
func (r *Registry) GetAllAliases() map[string]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]string, len(r.aliasToName))
	for alias, name := range r.aliasToName {
		result[alias] = name
	}
	return result
}
