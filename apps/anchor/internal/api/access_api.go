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
	body := request.Body
	question := askAbout(request.ProductId, body.Subject, body.Resource).Can(body.Action.Name)
	if body.Options != nil && functional.FromPtr(body.Options.Explain).OrElse(false) {
		question = question.Explaining()
	}

	decision, err := s.AccessEvaluator.Evaluate(ctx, question)
	if err != nil {
		return nil, err
	}
	result := decisionContext(decision)
	if len(decision.Items) > 0 {
		result.GrantedBy = grantedBy(decision.Items[0])
	}
	return EvaluateAccess200JSONResponse{Decision: decision.Allowed, Context: result}, nil
}

// EvaluateAccessBatch decides several actions for one subject and resource.
func (s *AnchorAPI) EvaluateAccessBatch(
	ctx context.Context,
	request EvaluateAccessBatchRequestObject,
) (EvaluateAccessBatchResponseObject, error) {
	body := request.Body
	actions, err := batchActions(*body)
	if err != nil {
		return nil, err
	}
	question := askAbout(request.ProductId, body.Subject, body.Resource).
		Can(actions...).
		Stopping(batchSemantic(body.Options))
	if body.Options != nil && functional.FromPtr(body.Options.Explain).OrElse(false) {
		question = question.Explaining()
	}

	decision, err := s.AccessEvaluator.Evaluate(ctx, question)
	if err != nil {
		return nil, err
	}
	return EvaluateAccessBatch200JSONResponse{
		Decision:    decision.Allowed,
		Context:     decisionContext(decision),
		Evaluations: functional.Slice(decision.Items).Map(itemDecision),
	}, nil
}

func askAbout(productID string, subject AccessSubject, resource AccessResource) accesssvc.Request {
	question := accesssvc.Ask(productID).As(accesssvc.SubjectType(subject.Type), subject.Id)
	properties := functional.FromPtr(resource.Properties).OrElse(AccessResourceProperties{})
	question = question.On(
		accesssvc.ResourceType(resource.Type),
		functional.FromPtr(resource.Id).OrElse(""),
		functional.FromPtr(properties.OrganizationId).OrElse(""),
	)
	if parent := properties.Parent; parent != nil {
		question = question.Under(parent.Type, parent.Id)
	}
	return question
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
		action := functional.OptionOf(item.Action, item.Action != nil).OrElse(body.Action)
		if action == nil {
			return nil, fault.BadRequest("INVALID_ACCESS_ACTION",
				"Each evaluation needs an action, or a top-level action as its default.")
		}
		actions = append(actions, action.Name)
	}
	return actions, nil
}

func batchSemantic(options *AccessEvaluationsOptions) accesssvc.Semantic {
	if options == nil || options.EvaluationsSemantic == nil {
		return accesssvc.ExecuteAll
	}
	return accesssvc.Semantic(*options.EvaluationsSemantic)
}

func itemDecision(item accesssvc.ItemDecision) AccessItemDecision {
	if !item.Allowed {
		return AccessItemDecision{Context: &AccessDecisionContext{Reason: denyReason(item.Reason)}}
	}
	if granted := grantedBy(item); granted != nil {
		return AccessItemDecision{Decision: true, Context: &AccessDecisionContext{GrantedBy: granted}}
	}
	return AccessItemDecision{Decision: true}
}

// grantedBy is nil when the request asked for no explanation, so the field is
// left out, and an empty list when the tables show no grant yet.
func grantedBy(item accesssvc.ItemDecision) *[]AccessGrant {
	if item.GrantedBy == nil {
		return nil
	}
	var grants []AccessGrant = functional.Slice(item.GrantedBy).Map(func(grant accesssvc.Grant) AccessGrant {
		return AccessGrant{
			Role:     identityRef(grant.RoleID, grant.RoleName),
			ApiKey:   identityRef(grant.APIKeyID, grant.APIKeyName),
			Resource: AccessGrantResource{Type: grant.Resource.Type, Id: grant.Resource.ID},
		}
	})
	return &grants
}

// decisionContext carries the deny reason and, when the subject reached the
// organization, who it resolved to and what it holds.
func decisionContext(decision accesssvc.Decision) AccessDecisionContext {
	result := AccessDecisionContext{Reason: denyReason(decision.Reason)}
	if len(decision.MissingPermissions) > 0 {
		result.MissingPermissions = &decision.MissingPermissions
	}
	identity := decision.Identity
	if identity == nil {
		return result
	}
	permissions := append([]string{}, identity.Permissions...)
	result.Permissions = &permissions
	result.Organization = identityRef(identity.OrganizationID, identity.OrganizationName)
	result.Role = identityRef(identity.RoleID, identity.RoleName)
	result.ProductUser = identityRef(identity.ProductUserID, identity.ProductUserName)
	if email := identity.ProductUserEmail; result.ProductUser != nil {
		result.ProductUser.Email = functional.OptionOf(email, email != "").ToPtr()
	}
	result.ApiKey = identityRef(identity.APIKeyID, identity.APIKeyName)
	return result
}

func denyReason(reason accesssvc.DenyReason) *AccessDenyReason {
	if reason == "" {
		return nil
	}
	wire := AccessDenyReason(reason)
	return &wire
}

// identityRef is nil for an absent identity, so the field is left out.
func identityRef(id, name string) *AccessIdentityRef {
	if id == "" {
		return nil
	}
	return &AccessIdentityRef{Id: id, Name: functional.OptionOf(name, name != "").ToPtr()}
}
