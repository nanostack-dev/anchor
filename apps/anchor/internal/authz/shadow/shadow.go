// Package shadow runs every access decision on both evaluators (ADR-0022 G4,
// PR 6): the decision always comes from the current evaluator, and OpenFGA's
// answer for each action it evaluated is compared with it, off the request
// path. A difference is logged as authz.shadow_mismatch and counted, so the
// store can be proven to agree before decisions move to it.
package shadow

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/rs/zerolog"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/fga"
	"anchor/internal/authz/model"
	"anchor/internal/authz/store"
)

const (
	// timeout bounds the OpenFGA side of one decision.
	timeout = 500 * time.Millisecond
	// workers bounds the comparisons running at once; a decision that finds
	// every slot taken is not compared, and counted as dropped.
	workers = 8
	// A drift that hits every decision logs this many mismatches a minute.
	logBurst = 20
)

type evaluator struct {
	inner   accessservice.Evaluator
	stores  store.Stores
	stats   *Stats
	slots   chan struct{}
	sampler zerolog.Sampler
}

// Stats counts what the shadow compared since start; tests read it.
type Stats struct {
	compared   atomic.Int64
	mismatches atomic.Int64
	failed     atomic.Int64
	skipped    atomic.Int64
	dropped    atomic.Int64
	running    sync.WaitGroup
}

func NewStats() *Stats { return &Stats{} }

// Compared counts the actions OpenFGA answered and the shadow compared.
func (s *Stats) Compared() int64 { return s.compared.Load() }

// Mismatches counts the compared actions OpenFGA answered differently.
func (s *Stats) Mismatches() int64 { return s.mismatches.Load() }

// Failed counts the decisions OpenFGA could not answer: a failed read or
// check, or an item it failed on.
func (s *Stats) Failed() int64 { return s.failed.Load() }

// Skipped counts the decisions not compared because the engine was not ready
// or the Product had no model yet; the first rebuild gives it one.
func (s *Stats) Skipped() int64 { return s.skipped.Load() }

// Dropped counts the decisions not compared because every worker was busy.
func (s *Stats) Dropped() int64 { return s.dropped.Load() }

// Settle waits for the comparisons still running.
func (s *Stats) Settle() { s.running.Wait() }

// Decorate puts the shadow around the current evaluator.
func Decorate(inner accessservice.Evaluator, stores store.Stores, stats *Stats) accessservice.Evaluator {
	return &evaluator{
		inner: inner, stores: stores, stats: stats,
		slots:   make(chan struct{}, workers),
		sampler: &zerolog.BurstSampler{Burst: logBurst, Period: time.Minute},
	}
}

func (e *evaluator) Evaluate(ctx context.Context, request accessservice.Request) (accessservice.Decision, error) {
	decision, err := e.inner.Evaluate(ctx, request)
	if err != nil || decision.Identity == nil {
		return decision, err
	}
	select {
	case e.slots <- struct{}{}:
	default:
		e.stats.dropped.Add(1)
		return decision, nil
	}
	e.stats.running.Add(1)
	go func() {
		defer func() {
			<-e.slots
			e.stats.running.Done()
		}()
		compareCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
		defer cancel()
		e.compare(compareCtx, request, decision)
	}()
	return decision, nil
}

// compare asks OpenFGA about every action the current evaluator evaluated, for
// the identity it reached. Only a reached identity is compared: a subject the
// evaluator stops earlier (unknown user, invalid or inactive key, not a
// member, unknown Organization or workspace) is still stopped in Postgres
// after the switch, so OpenFGA never answers for it.
func (e *evaluator) compare(ctx context.Context, request accessservice.Request, decision accessservice.Decision) {
	logger := log.Ctx(ctx).With().
		Str("component", "authz_shadow").
		Str("product_id", request.ProductID).
		Str("resource_type", string(request.Resource.Type)).
		Str("resource_id", request.Resource.ID).
		Logger().Sample(e.sampler)
	found, err := e.stores.Open(ctx, request.ProductID)
	if errors.Is(err, engine.ErrNotReady) {
		e.stats.skipped.Add(1)
		return
	}
	if err != nil {
		e.stats.failed.Add(1)
		logger.Warn().Err(err).Msg("authz shadow could not open the store")
		return
	}
	opened, ok := found.Get()
	if !ok {
		e.stats.skipped.Add(1)
		return
	}
	identity := decision.Identity
	subject := model.User(identity.ProductUserID)
	if identity.APIKeyID != "" {
		subject = model.APIKey(identity.APIKeyID)
	}
	var (
		indexes []int
		checks  []fga.Tuple
	)
	for i, item := range decision.Items {
		if item.Reason != accessservice.ReasonNotEvaluated && i < len(request.Actions) {
			indexes = append(indexes, i)
			checks = append(checks, fga.Tuple{
				Object:   model.Organization(identity.OrganizationID),
				Relation: model.CanRelation(request.Actions[i]),
				User:     subject,
			})
		}
	}
	answers, err := opened.Holds(ctx, checks)
	if err != nil {
		e.stats.failed.Add(1)
		logger.Warn().Err(err).Msg("authz shadow check failed")
		return
	}
	for n, i := range indexes {
		e.stats.compared.Add(1)
		legacy := decision.Items[i].Allowed
		if answers[n] == legacy {
			continue
		}
		e.stats.mismatches.Add(1)
		logger.Warn().
			Str("event", "authz.shadow_mismatch").
			Str("organization_id", identity.OrganizationID).
			Str("subject", subject).
			Str("permission", request.Actions[i]).
			Bool("legacy", legacy).
			Bool("openfga", answers[n]).
			Msg("authz.shadow_mismatch")
	}
}
