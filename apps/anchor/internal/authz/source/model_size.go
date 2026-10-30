package source

import (
	"context"
	"fmt"
	"sync"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/pgkit/pglock"

	"anchor/internal/authz/model"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	rtrepository "anchor/internal/resourcetype/repository"
)

// CodeModelTooLarge is the code of NewModelTooLargeError.
const CodeModelTooLarge = "AUTHZ_MODEL_TOO_LARGE"

// maxFittingHashes bounds the hashes ModelSizeGuard remembers; past it, it
// starts over.
const maxFittingHashes = 256

// NewModelTooLargeError refuses a catalog write whose model OpenFGA would not
// store.
func NewModelTooLargeError(modelBytes int) *fault.Error {
	return fault.Conflict(
		CodeModelTooLarge,
		fmt.Sprintf(
			"With this change, the product's authorization model would be %d bytes. The limit is %d bytes. "+
				"Delete permissions or resource types first.",
			modelBytes, model.MaxBytes,
		),
	).Metadata(map[string]any{"model_bytes": modelBytes, "max_model_bytes": model.MaxBytes})
}

// ModelSizeGuard compiles the catalog inside each write that can grow it and
// refuses the write when OpenFGA would refuse the model. Without it, the write
// commits, every sync of the Product fails, and its grants stop syncing.
//
// It runs inside every write that emits an event, so it depends on the tables
// only: nothing it needs may itself emit events.
type ModelSizeGuard struct {
	tables        Tables
	resourceTypes rtrepository.Repository
	mu            sync.Mutex
	fits          map[string]bool
}

func NewModelSizeGuard(tables Tables, resourceTypes rtrepository.Repository) *ModelSizeGuard {
	return &ModelSizeGuard{tables: tables, resourceTypes: resourceTypes, fits: map[string]bool{}}
}

// ObserveTx checks the catalog as the write leaves it. Deletes and a
// permission's description change never grow the model, so a Product above
// the limit can always shrink back below it.
func (g *ModelSizeGuard) ObserveTx(ctx context.Context, event events.Event) error {
	if !isOneOf(event, events.ProductResourceTypeCreated, events.ProductResourceTypeUpdated,
		events.ProductResourcePermissionCreated) {
		return nil
	}
	// Permission writes of one Product run side by side: each must see the
	// others' committed rows before it measures.
	if _, err := transactor.Executor(ctx, g.tables.db).ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`,
		pglock.KeyHash("authz-model-size:"+event.ProductID)); err != nil {
		return err
	}
	types, err := g.resourceTypes.FindTree(ctx, event.ProductID)
	if err != nil {
		return err
	}
	catalog, err := g.tables.catalog(ctx, event.ProductID, resourcetype.NewTree(types))
	if err != nil {
		return err
	}
	// A tree replacement emits one event per type; compile its catalog once.
	hash := model.Hash(catalog)
	if g.fitting(hash) {
		return nil
	}
	compiled, err := model.Compile(catalog)
	if err != nil {
		return err
	}
	if size := compiled.Bytes(); size > model.MaxBytes {
		return NewModelTooLargeError(size)
	}
	g.remember(hash)
	return nil
}

func (g *ModelSizeGuard) fitting(hash string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.fits[hash]
}

func (g *ModelSizeGuard) remember(hash string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.fits) >= maxFittingHashes {
		g.fits = map[string]bool{}
	}
	g.fits[hash] = true
}
