package api

import (
	"context"

	accesssvc "anchor/internal/access/service"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

// EvaluateAccess decides one action, following the AuthZEN access evaluation shape.
func (s *AnchorAPI) EvaluateAccess(
	ctx context.Context,
	request EvaluateAccessRequestObject,
) (EvaluateAccessResponseObject, error) {
	decision, err := s.AccessEvaluator.Evaluate(ctx, accesssvc.EvaluateInput{
		ProductID: request.ProductId,
		Subject:   toAccessSubject(request.Body.Subject),
		Resource:  toAccessResource(request.Body.Resource),
		Actions:   []string{request.Body.Action.Name},
	})
	if err != nil {
		return nil, err
	}
	return EvaluateAccess200JSONResponse{
		Decision: decision.Allowed,
		Context:  toAccessDecisionContext(decision),
	}, nil
}

// EvaluateAccessBatch decides several actions for one subject and resource.
func (s *AnchorAPI) EvaluateAccessBatch(
	ctx context.Context,
	request EvaluateAccessBatchRequestObject,
) (EvaluateAccessBatchResponseObject, error) {
	actions, err := batchActions(*request.Body)
	if err != nil {
		return nil, err
	}
	semantic := functional.FromPtr(request.Body.Options).
		Map(func(options AccessEvaluationsOptions) accesssvc.Semantic {
			return accesssvc.Semantic(functional.FromPtr(options.EvaluationsSemantic).OrElse(ExecuteAll))
		}).
		OrElse(accesssvc.ExecuteAll)
	decision, err := s.AccessEvaluator.Evaluate(ctx, accesssvc.EvaluateInput{
		ProductID: request.ProductId,
		Subject:   toAccessSubject(request.Body.Subject),
		Resource:  toAccessResource(request.Body.Resource),
		Actions:   actions,
		Semantic:  semantic,
	})
	if err != nil {
		return nil, err
	}
	return EvaluateAccessBatch200JSONResponse{
		Decision: decision.Allowed,
		Context:  toAccessDecisionContext(decision),
		Evaluations: functional.Slice(decision.Items).Map(func(item accesssvc.ItemDecision) AccessItemDecision {
			result := AccessItemDecision{Decision: item.Allowed}
			if !item.Allowed {
				reason := AccessDenyReason(item.Reason)
				result.Context = &AccessDecisionContext{Reason: &reason}
			}
			return result
		}),
	}, nil
}

// batchActions applies AuthZEN's batch defaults: a top-level action fills items
// without one and, with no items, is evaluated alone. Overrides that would
// change the subject or resource are refused, never dropped: ignoring one would
// decide a different question than the caller asked.
func batchActions(body AccessEvaluationsRequest) ([]string, error) {
	items := functional.FromPtr(body.Evaluations).OrElse(nil)
	if len(items) == 0 {
		if body.Action != nil {
			return []string{body.Action.Name}, nil
		}
		return []string{}, nil
	}
	actions := make([]string, 0, len(items))
	for _, item := range items {
		if item.Subject != nil || item.Resource != nil {
			return nil, fault.BadRequest("UNSUPPORTED_ACCESS_OVERRIDE",
				"Per-item subject and resource overrides are not supported; send one request per subject and resource.")
		}
		action := item.Action
		if action == nil {
			action = body.Action
		}
		if action == nil {
			return nil, fault.BadRequest(
				"INVALID_ACCESS_ACTION",
				"Each evaluation needs an action, or a top-level action as its default.",
			)
		}
		actions = append(actions, action.Name)
	}
	return actions, nil
}

func toAccessSubject(subject AccessSubject) accesssvc.Subject {
	return accesssvc.Subject{Type: accesssvc.SubjectType(subject.Type), ID: subject.Id}
}

func toAccessResource(resource AccessResource) accesssvc.Resource {
	result := accesssvc.Resource{
		Type: accesssvc.ResourceType(resource.Type),
		ID:   functional.FromPtr(resource.Id).OrElse(""),
	}
	if resource.Properties != nil {
		result.OrganizationID = functional.FromPtr(resource.Properties.OrganizationId).OrElse("")
	}
	return result
}

func toAccessDecisionContext(decision accesssvc.Decision) AccessDecisionContext {
	result := AccessDecisionContext{}
	if decision.Reason != "" {
		reason := AccessDenyReason(decision.Reason)
		result.Reason = &reason
	}
	if len(decision.MissingPermissions) > 0 {
		result.MissingPermissions = &decision.MissingPermissions
	}
	identity := decision.Identity
	if identity == nil {
		return result
	}
	result.Organization = &AccessIdentityRef{Id: identity.OrganizationID, Name: optionalName(identity.OrganizationName)}
	permissions := append([]string{}, identity.Permissions...)
	result.Permissions = &permissions
	if identity.RoleID != "" {
		result.Role = &AccessIdentityRef{Id: identity.RoleID, Name: optionalName(identity.RoleName)}
	}
	if identity.ProductUserID != "" {
		result.ProductUser = &AccessIdentityRef{
			Id:   identity.ProductUserID,
			Name: optionalName(identity.ProductUserName),
		}
	}
	if identity.APIKeyID != "" {
		result.ApiKey = &AccessIdentityRef{Id: identity.APIKeyID}
	}
	return result
}

func optionalName(name string) *string {
	return functional.OptionOf(name, name != "").ToPtr()
}
