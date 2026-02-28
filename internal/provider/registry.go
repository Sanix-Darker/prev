package provider

import (
	"fmt"
	"sort"
	"sync"

	"github.com/sanix-darker/prev/internal/config"
)

// ---------------------------------------------------------------------------
// Provider factory
// ---------------------------------------------------------------------------

// Factory is a constructor function that creates an AIProvider from a config
// store subtree. Each provider registers its own factory.
//
// The store is scoped to the provider's configuration block, e.g.:
//
//	providers:
//	  openai:
//	    api_key: sk-...
//	    model: gpt-4o
//
// would pass a store that resolves "api_key" and "model" directly.
type Factory func(v *config.Store) (AIProvider, error)

// ---------------------------------------------------------------------------
// Registry
// ---------------------------------------------------------------------------

// Registry is a thread-safe store of provider factories. It implements the
// factory / service-locator pattern so that provider implementations
// self-register at init() time and the application can resolve them by name.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// globalRegistry is the package-level registry used by the convenience
// functions Register / Get / List / MustGet.
var globalRegistry = NewRegistry()

// NewRegistry creates an empty Registry. Useful for testing.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a provider factory under the given name. It panics if the
// name is already registered, preventing silent overwrites.
//
// Provider packages should call the package-level Register() in their init():
//
//	func init() {
//	    provider.Register("openai", NewOpenAIProvider)
//	}
func (r *Registry) Register(name string, f Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		panic(fmt.Sprintf("provider: factory already registered for %q", name))
	}
	r.factories[name] = f
}

// Get creates a provider instance by name using the given config.
func (r *Registry) Get(name string, v *config.Store) (AIProvider, error) {
	r.mu.RLock()
	f, exists := r.factories[name]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("provider: unknown provider %q (registered: %v)",
			name, r.Names())
	}
	return f(v)
}

// Names returns a sorted list of registered provider names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ---------------------------------------------------------------------------
// Package-level convenience functions (delegate to globalRegistry)
// ---------------------------------------------------------------------------

// Register adds a provider factory to the global registry.
func Register(name string, f Factory) {
	globalRegistry.Register(name, f)
}

// Get resolves a provider by name from the global registry.
func Get(name string, v *config.Store) (AIProvider, error) {
	return globalRegistry.Get(name, v)
}

// Names returns all registered provider names from the global registry.
func Names() []string {
	return globalRegistry.Names()
}

// MustGet is like Get but panics on error. Useful for tests.
func MustGet(name string, v *config.Store) AIProvider {
	p, err := Get(name, v)
	if err != nil {
		panic(err)
	}
	return p
}
