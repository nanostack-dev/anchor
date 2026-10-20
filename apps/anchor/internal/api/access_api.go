package api

import (
	"context"

	accesssvc "anchor/internal/access/service"

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
	semantic := functional.FromPtr(request.Body.Options).
		Map(func(options AccessEvaluationsOptions) accesssvc.Semantic {
			return accesssvc.Semantic(functional.FromPtr(options.EvaluationsSemantic).OrElse(ExecuteAll))
		}).
		OrElse(accesssvc.ExecuteAll)
	decision, err := s.AccessEvaluator.Evaluate(ctx, accesssvc.EvaluateInput{
		ProductID: request.ProductId,
		Subject:   toAccessSubject(request.Body.Subject),
		Resource:  toAccessResource(request.Body.Resource),
		Actions: functional.Slice(request.Body.Evaluations).Map(func(item AccessEvaluationItem) string {
			return item.Action.Name
		}),
		Semantic: semantic,
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
