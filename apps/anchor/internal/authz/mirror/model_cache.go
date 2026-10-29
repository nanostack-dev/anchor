package mirror

import (
	"fmt"
	"sync"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"

	"anchor/internal/authz/fga"
)

// maxCachedModels bounds the cache; past it, the cache starts over.
const maxCachedModels = 256

type compiledModel struct {
	proto     *openfgav1.AuthorizationModel
	relations fga.Relations
}

// modelCache compiles each model hash once per process: every sync needs the
// model's relations, and compiling is far slower than hashing.
type modelCache struct {
	mu     sync.Mutex
	byHash map[string]compiledModel
}

func newModelCache() *modelCache {
	return &modelCache{byHash: map[string]compiledModel{}}
}

func (c *modelCache) get(source Model) (compiledModel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, found := c.byHash[source.Hash]; found {
		return cached, nil
	}
	proto, err := source.Compile()
	if err != nil {
		return compiledModel{}, fmt.Errorf("compile authz model: %w", err)
	}
	if len(c.byHash) >= maxCachedModels {
		c.byHash = map[string]compiledModel{}
	}
	compiled := compiledModel{proto: proto, relations: fga.RelationsOf(proto)}
	c.byHash[source.Hash] = compiled
	return compiled, nil
}
