package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/rs/zerolog"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
)

const (
	// shadowTimeout bounds the OpenFGA side of one decision.
	shadowTimeout = 500 * time.Millisecond
	// shadowWorkers bounds the comparisons running at once; a decision that
	// finds every slot taken is not compared, and counted as dropped.
	shadowWorkers = 8
	// shadowCheckBatch is OpenFGA's default limit of checks in one BatchCheck.
	shadowCheckBatch = 50
	// A drift that hits every decision logs this many mismatches a minute.
	shadowLogBurst = 20
)

// errShadowItem reports an item OpenFGA could not answer.
var errShadowItem = errors.New("authz shadow item failed")

// Shadow runs every decision on both evaluators (ADR-0022 G4, PR 6): the
// decision always comes from the current evaluator, and OpenFGA's answer for
// each action it evaluated is compared with it, off the request path. A
// difference is logged as authz.shadow_mismatch and counted, so the store can
// be proven to agree before decisions move to it.
type Shadow struct {
	inner   accessservice.Evaluator
	engines engine.Provider
	repo    repository.Repository
	stats   *ShadowStats
	slots   chan struct{}
	sampler zerolog.Sampler
}

// ShadowStats counts what the shadow compared since start; tests read it.
type ShadowStats struct {
	compared   atomic.Int64
	mismatches atomic.Int64
	failed     atomic.Int64
	skipped    atomic.Int64
	dropped    atomic.Int64
	running    sync.WaitGroup
}

func NewShadowStats() *ShadowStats { return &ShadowStats{} }

// Compared counts the actions OpenFGA answered and the shadow compared.
func (s *ShadowStats) Compared() int64 { return s.compared.Load() }

// Mismatches counts the compared actions OpenFGA answered differently.
func (s *ShadowStats) Mismatches() int64 { return s.mismatches.Load() }

// Failed counts the decisions OpenFGA could not answer: a failed read or
// check, or an item it failed on.
func (s *ShadowStats) Failed() int64 { return s.failed.Load() }

// Skipped counts the decisions not compared because the engine was not ready
// or the Product had no model yet; the first rebuild gives it one.
func (s *ShadowStats) Skipped() int64 { return s.skipped.Load() }

// Dropped counts the decisions not compared because every worker was busy.
func (s *ShadowStats) Dropped() int64 { return s.dropped.Load() }

// Settle waits for the comparisons still running.
func (s *ShadowStats) Settle() { s.running.Wait() }

// DecorateEvaluator puts the shadow around the current evaluator.
func DecorateEvaluator(
	inner accessservice.Evaluator,
	engines engine.Provider,
	repo repository.Repository,
	stats *ShadowStats,
) accessservice.Evaluator {
	return &Shadow{
		inner: inner, engines: engines, repo: repo, stats: stats,
		slots:   make(chan struct{}, shadowWorkers),
		sampler: &zerolog.BurstSampler{Burst: shadowLogBurst, Period: time.Minute},
	}
}

func (s *Shadow) Evaluate(ctx context.Context, request accessservice.Request) (accessservice.Decision, error) {
	decision, err := s.inner.Evaluate(ctx, request)
	if err != nil || decision.Identity == nil {
		return decision, err
	}
	select {
	case s.slots <- struct{}{}:
	default:
		s.stats.dropped.Add(1)
		return decision, nil
	}
	s.stats.running.Add(1)
	go func() {
		defer func() {
			<-s.slots
			s.stats.running.Done()
		}()
		compareCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shadowTimeout)
		defer cancel()
		s.compare(compareCtx, request, decision)
	}()
	return decision, nil
}

// compare asks OpenFGA about every action the current evaluator evaluated, for
// the identity it reached. Only a reached identity is compared: a subject the
// evaluator stops earlier (unknown user, invalid or inactive key, not a
// member, unknown Organization or workspace) is still stopped in Postgres
// after the switch, so OpenFGA never answers for it.
func (s *Shadow) compare(ctx context.Context, request accessservice.Request, decision accessservice.Decision) {
	logger := log.Ctx(ctx).With().
		Str("component", "authz_shadow").
		Str("product_id", request.ProductID).
		Str("resource_type", string(request.Resource.Type)).
		Str("resource_id", request.Resource.ID).
		Logger().Sample(s.sampler)
	openfga, err := s.engines.Get()
	if err != nil {
		s.stats.skipped.Add(1)
		return
	}
	found, err := s.repo.FindStore(ctx, request.ProductID)
	if err != nil {
		s.stats.failed.Add(1)
		logger.Warn().Err(err).Msg("authz shadow could not read the store")
		return
	}
	stored, ok := found.Get()
	if !ok || stored.ModelID == "" {
		s.stats.skipped.Add(1)
		return
	}
	identity := decision.Identity
	subject := model.User(identity.ProductUserID)
	if identity.APIKeyID != "" {
		subject = model.APIKey(identity.APIKeyID)
	}
	var indexes []int
	for i, item := range decision.Items {
		if item.Reason != accessservice.ReasonNotEvaluated && i < len(request.Actions) {
			indexes = append(indexes, i)
		}
	}
	for start := 0; start < len(indexes); start += shadowCheckBatch {
		batch := indexes[start:min(start+shadowCheckBatch, len(indexes))]
		answers, checkErr := s.check(ctx, openfga, stored, subject, identity.OrganizationID, request.Actions, batch)
		if checkErr != nil {
			s.stats.failed.Add(1)
			logger.Warn().Err(checkErr).Msg("authz shadow check failed")
			return
		}
		for _, i := range batch {
			s.stats.compared.Add(1)
			legacy := decision.Items[i].Allowed
			if answers[i] == legacy {
				continue
			}
			s.stats.mismatches.Add(1)
			logger.Warn().
				Str("event", "authz.shadow_mismatch").
				Str("organization_id", identity.OrganizationID).
				Str("subject", subject).
				Str("permission", request.Actions[i]).
				Bool("legacy", legacy).
				Bool("openfga", answers[i]).
				Msg("authz.shadow_mismatch")
		}
	}
}

// check answers the actions at the given indexes in one BatchCheck. A
// permission the model doesn't know is not held, as it will be after the
// switch; an item OpenFGA fails on fails the comparison.
func (s *Shadow) check(
	ctx context.Context, openfga engine.Engine, stored repository.Store,
	subject, organizationID string, actions []string, indexes []int,
) (map[int]bool, error) {
	checks := make([]*openfgav1.BatchCheckItem, len(indexes))
	for n, i := range indexes {
		checks[n] = &openfgav1.BatchCheckItem{
			CorrelationId: strconv.Itoa(i),
			TupleKey: &openfgav1.CheckRequestTupleKey{
				User:     subject,
				Relation: model.CanRelation(actions[i]),
				Object:   model.Organization(organizationID),
			},
		}
	}
	resp, err := openfga.BatchCheck(ctx, &openfgav1.BatchCheckRequest{
		StoreId: stored.StoreID, AuthorizationModelId: stored.ModelID, Checks: checks,
	})
	if err != nil {
		return nil, err
	}
	answers := make(map[int]bool, len(indexes))
	for _, i := range indexes {
		result, present := resp.GetResult()[strconv.Itoa(i)]
		if !present || result.GetError().GetInternalError() != openfgav1.InternalErrorCode_no_internal_error {
			return nil, fmt.Errorf("%w: %s", errShadowItem, result.GetError().GetMessage())
		}
		answers[i] = result.GetAllowed()
	}
	return answers, nil
}
