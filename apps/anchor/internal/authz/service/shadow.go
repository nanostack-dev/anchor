package service

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/rs/zerolog"

	accessservice "anchor/internal/access/service"
	"anchor/internal/authz/engine"
	"anchor/internal/authz/model"
	"anchor/internal/authz/repository"
)

// shadowTimeout bounds the OpenFGA side of one decision.
const shadowTimeout = 2 * time.Second

// Shadow runs every decision on both evaluators (ADR-0022 G4, PR 6): the
// decision always comes from the current evaluator, and OpenFGA's answer for
// each action it evaluated is compared with it. A difference is logged as
// authz.shadow_mismatch and counted, so the store can be proven to agree
// before decisions move to it.
type Shadow struct {
	inner   accessservice.Evaluator
	engines engine.Provider
	repo    repository.Repository
	stats   *ShadowStats
	logger  zerolog.Logger
}

// ShadowStats counts the differences seen since start; tests read it.
type ShadowStats struct {
	mismatches atomic.Int64
}

func NewShadowStats() *ShadowStats { return &ShadowStats{} }

func (s *ShadowStats) Mismatches() int64 { return s.mismatches.Load() }

// DecorateEvaluator puts the shadow around the current evaluator.
func DecorateEvaluator(
	inner accessservice.Evaluator,
	engines engine.Provider,
	repo repository.Repository,
	stats *ShadowStats,
	logger zerolog.Logger,
) accessservice.Evaluator {
	return &Shadow{
		inner: inner, engines: engines, repo: repo, stats: stats,
		logger: logger.With().Str("component", "authz_shadow").Logger(),
	}
}

func (s *Shadow) Evaluate(ctx context.Context, request accessservice.Request) (accessservice.Decision, error) {
	decision, err := s.inner.Evaluate(ctx, request)
	if err == nil && decision.Identity != nil {
		s.compare(ctx, request, decision)
	}
	return decision, err
}

// compare asks OpenFGA about every action the current evaluator evaluated, for
// the identity it reached. Only a reached identity is compared: a subject the
// evaluator stops earlier (unknown user, invalid key, not a member) never
// reaches OpenFGA after the switch either.
func (s *Shadow) compare(ctx context.Context, request accessservice.Request, decision accessservice.Decision) {
	openfga, err := s.engines.Get()
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shadowTimeout)
	defer cancel()
	found, err := s.repo.FindStore(ctx, request.ProductID)
	if err != nil {
		s.logger.Warn().Err(err).Str("product_id", request.ProductID).Msg("authz shadow could not read the store")
		return
	}
	stored, ok := found.Get()
	if !ok || stored.ModelID == "" {
		return
	}
	identity := decision.Identity
	subject := model.User(identity.ProductUserID)
	if identity.APIKeyID != "" {
		subject = model.APIKey(identity.APIKeyID)
	}
	var checks []*openfgav1.BatchCheckItem
	var indexes []int
	for i, item := range decision.Items {
		if item.Reason == accessservice.ReasonNotEvaluated || i >= len(request.Actions) {
			continue
		}
		checks = append(checks, &openfgav1.BatchCheckItem{
			CorrelationId: strconv.Itoa(i),
			TupleKey: &openfgav1.CheckRequestTupleKey{
				User:     subject,
				Relation: model.CanRelation(request.Actions[i]),
				Object:   model.Organization(identity.OrganizationID),
			},
		})
		indexes = append(indexes, i)
	}
	if len(checks) == 0 {
		return
	}
	resp, err := openfga.BatchCheck(ctx, &openfgav1.BatchCheckRequest{
		StoreId: stored.StoreID, AuthorizationModelId: stored.ModelID, Checks: checks,
	})
	if err != nil {
		s.logger.Warn().Err(err).Str("product_id", request.ProductID).Msg("authz shadow check failed")
		return
	}
	for _, i := range indexes {
		result, present := resp.GetResult()[strconv.Itoa(i)]
		if !present || result.GetError() != nil {
			continue
		}
		legacy := decision.Items[i].Allowed
		if result.GetAllowed() == legacy {
			continue
		}
		s.stats.mismatches.Add(1)
		s.logger.Warn().
			Str("event", "authz.shadow_mismatch").
			Str("product_id", request.ProductID).
			Str("organization_id", identity.OrganizationID).
			Str("subject", subject).
			Str("permission", request.Actions[i]).
			Bool("legacy", legacy).
			Bool("openfga", result.GetAllowed()).
			Msg("authz.shadow_mismatch")
	}
}
