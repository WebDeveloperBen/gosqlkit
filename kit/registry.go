package kit

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Provider interface {
	Dialect() DialectInfo
	RenderSQL() (string, error)
	SnapshotJSON() ([]byte, error)
}

type DialectInfo struct {
	Name         string
	Aliases      []string
	Capabilities Capabilities
}

type Capabilities struct {
	Tables          bool
	Schemas         bool
	Extensions      bool
	Enums           bool
	ForeignKeys     bool
	Checks          bool
	Indexes         bool
	AdvancedIndexes bool
	Snapshots       bool
}

var registry = struct {
	providers map[string]Provider
	aliases   map[string]string
	infos     map[string]DialectInfo
	sync.RWMutex
}{
	providers: map[string]Provider{},
	aliases:   map[string]string{},
	infos:     map[string]DialectInfo{},
}

func Register(provider Provider) {
	registry.Lock()
	defer registry.Unlock()

	if provider == nil {
		panic("kit: provider must not be nil")
	}

	info := normaliseInfo(provider.Dialect())
	if info.Name == "" {
		panic("kit: dialect name must not be empty")
	}

	names := append([]string{info.Name}, info.Aliases...)
	for _, name := range names {
		if existing, ok := registry.aliases[name]; ok {
			panic(fmt.Sprintf("kit: dialect name %q is already registered for %q", name, existing))
		}
	}

	registry.providers[info.Name] = provider
	registry.infos[info.Name] = info
	for _, name := range names {
		registry.aliases[name] = info.Name
	}
}

func RenderSQL(dialect string) (string, error) {
	provider, err := provider(dialect)
	if err != nil {
		return "", err
	}
	return provider.RenderSQL()
}

func SnapshotJSON(dialect string) ([]byte, error) {
	provider, err := provider(dialect)
	if err != nil {
		return nil, err
	}
	return provider.SnapshotJSON()
}

func Dialects() []string {
	registry.RLock()
	defer registry.RUnlock()

	return dialectsLocked()
}

func dialectsLocked() []string {
	dialects := make([]string, 0, len(registry.providers))
	for dialect := range registry.providers {
		dialects = append(dialects, dialect)
	}
	sort.Strings(dialects)
	return dialects
}

func DialectInfos() []DialectInfo {
	registry.RLock()
	defer registry.RUnlock()

	names := make([]string, 0, len(registry.infos))
	for name := range registry.infos {
		names = append(names, name)
	}
	sort.Strings(names)

	infos := make([]DialectInfo, 0, len(names))
	for _, name := range names {
		infos = append(infos, cloneInfo(registry.infos[name]))
	}
	return infos
}

func DialectInfoFor(dialect string) (DialectInfo, error) {
	registry.RLock()
	defer registry.RUnlock()

	name := normaliseName(dialect)
	if name == "" {
		name = "postgres"
	}
	canonical, ok := registry.aliases[name]
	if !ok {
		return DialectInfo{}, unknownDialectError(name, dialectsLocked())
	}
	return cloneInfo(registry.infos[canonical]), nil
}

func provider(dialect string) (Provider, error) {
	registry.RLock()
	defer registry.RUnlock()

	name := normaliseName(dialect)
	if name == "" {
		name = "postgres"
	}
	canonical, ok := registry.aliases[name]
	if !ok {
		return nil, unknownDialectError(name, dialectsLocked())
	}
	return registry.providers[canonical], nil
}

func normaliseInfo(info DialectInfo) DialectInfo {
	info.Name = normaliseName(info.Name)

	seen := map[string]struct{}{info.Name: {}}
	aliases := make([]string, 0, len(info.Aliases))
	for _, alias := range info.Aliases {
		alias = normaliseName(alias)
		if alias == "" {
			panic("kit: dialect alias must not be empty")
		}
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		aliases = append(aliases, alias)
	}
	info.Aliases = aliases
	return info
}

func cloneInfo(info DialectInfo) DialectInfo {
	info.Aliases = append([]string(nil), info.Aliases...)
	return info
}

func normaliseName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func unknownDialectError(dialect string, registered []string) error {
	return fmt.Errorf("unknown dialect %q; registered dialects: %v", dialect, registered)
}
