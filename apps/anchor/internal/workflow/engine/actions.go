package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"anchor/internal/domain/email"
	"anchor/internal/domain/license"
	"anchor/internal/domain/organization"
	"anchor/internal/domain/organizationinvitation"
	"anchor/internal/domain/product/user"
	"anchor/internal/domain/workflow"
	"anchor/internal/domain/workspace"
	emailsvc "anchor/internal/email/service"
	"anchor/internal/events"
	invitationsvc "anchor/internal/invitation/service"
	licensesvc "anchor/internal/license/service"
	anchorservice "anchor/internal/service"
)

const (
	GroupOrganizations = "Organizations"
	GroupWorkspaces    = "Workspaces"
	GroupMembers       = "Members"
	GroupUsers         = "Users"
	GroupLicensing     = "Licensing"
	GroupEmail         = "Email"
	GroupCustom        = "Custom"
)

const (
	ActionOrganizationGet    workflow.ActionType = "organization.get"
	ActionOrganizationCreate workflow.ActionType = "organization.create"
	ActionOrganizationUpdate workflow.ActionType = "organization.update"
	ActionWorkspaceCreate    workflow.ActionType = "workspace.create"
	ActionMemberAdd          workflow.ActionType = "member.add"
	ActionMemberUpdateRole   workflow.ActionType = "member.update_role"
	ActionMemberRemove       workflow.ActionType = "member.remove"
	ActionInvitationCreate   workflow.ActionType = "invitation.create"
	ActionProductUserGet     workflow.ActionType = "product_user.get"
	ActionLicenseInstantiate workflow.ActionType = "license.instantiate"
	ActionLicenseMigrate     workflow.ActionType = "license.migrate"
	ActionLicenseAdjust      workflow.ActionType = "license.adjust"
	ActionEmailSend          workflow.ActionType = "email.send"
	ActionHTTPRequest        workflow.ActionType = "http.request"
	ActionWorkflowEmit       workflow.ActionType = "workflow.emit"
)

const (
	keyOrganizationID = "organization_id"
	keyProductUserID  = "product_user_id"
	keyRoleID         = "role_id"
	keyTemplateID     = "template_id"
	keyLicenseID      = "license_id"
	keyName           = "name"
	keyDescription    = "description"
	keyMetadata       = "metadata"
	keyEmail          = "email"
	keyStatus         = "status"
	keyEvent          = "event"

	labelName            = "Name"
	labelDescription     = "Description"
	labelLicenseTemplate = "License template"

	describeOrganizationID = "Identifier of the organization."
)

type Services struct {
	Organizations anchorservice.OrganizationService
	Workspaces    anchorservice.WorkspaceService
	Memberships   anchorservice.OrganizationMembershipService
	ProductUsers  anchorservice.ProductUserService
	Invitations   invitationsvc.OrganizationInvitationService
	Licenses      licensesvc.OrganizationLicenseService
	Migrations    licensesvc.LicenseMigrationService
	Email         emailsvc.EmailService
	CustomEvents  CustomEventSender
	Caller        *HTTPCaller
}

func buildActions(s Services) []action {
	organizationIDParam := ParamSpec{
		Name: keyOrganizationID, Label: "Organization", Type: ParamOrganization, Required: true,
	}
	productUserIDParam := ParamSpec{
		Name: keyProductUserID, Label: "Product user", Type: ParamProductUser, Required: true,
	}
	roleIDParam := ParamSpec{
		Name: keyRoleID, Label: "Role", Type: ParamRole, Required: true,
	}
	organizationOutputs := []OutputSpec{
		{Name: keyOrganizationID, Description: describeOrganizationID},
		{Name: keyName, Description: "Name of the organization."},
		{Name: keyDescription, Description: "Description of the organization."},
		{Name: keyMetadata, Description: "Metadata object; read a key with metadata.<key>."},
	}
	membershipOutputs := []OutputSpec{
		{Name: keyOrganizationID, Description: describeOrganizationID},
		{Name: keyProductUserID, Description: "Identifier of the member."},
		{Name: keyRoleID, Description: "Identifier of the member's role."},
		{Name: "role_name", Description: "Name of the member's role."},
	}
	return []action{
		{
			spec: ActionSpec{
				Type: ActionOrganizationGet, Group: GroupOrganizations, Name: "Read organization",
				Description: "Loads an organization so later steps and conditions can use its name and metadata.",
				Params:      []ParamSpec{organizationIDParam},
				Outputs:     organizationOutputs,
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				found, err := s.Organizations.Find(ctx, organization.FindOrganizationInput{
					ProductID: env.ProductID, OrganizationID: p.String(keyOrganizationID),
				})
				if err != nil {
					return nil, err
				}
				return organizationOutput(found), nil
			},
		},
		{
			spec: ActionSpec{
				Type:   ActionOrganizationCreate,
				Group:  GroupOrganizations,
				Name:   "Create organization",
				Writes: true,
				MayEmit: []string{
					string(events.OrganizationCreated),
					string(events.MembershipCreated),
					string(events.OrganizationLicenseUpdated),
				},
				Description: "Creates an organization. With an owner, the product user becomes its first member; " +
					"with a license template, the organization is licensed in the same write.",
				Params: []ParamSpec{
					{Name: keyName, Label: labelName, Type: ParamText, Required: true},
					{Name: keyDescription, Label: labelDescription, Type: ParamText},
					{
						Name:        keyMetadata,
						Label:       "Metadata",
						Type:        ParamJSON,
						Description: "JSON object stored on the organization.",
					},
					{Name: "owner_product_user_id", Label: "Owner", Type: ParamProductUser,
						Description: "Product user added as the first member. Needs an owner role."},
					{Name: "owner_role_id", Label: "Owner role", Type: ParamRole},
					{Name: "license_template_id", Label: labelLicenseTemplate, Type: ParamLicenseTemplate},
				},
				Outputs: organizationOutputs,
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				metadata, _, err := p.Object(keyMetadata)
				if err != nil {
					return nil, err
				}
				owner := p.String("owner_product_user_id")
				if owner == "" {
					created, createErr := s.Organizations.Create(ctx, organization.CreateOrganizationInput{
						TenantID: env.TenantID, ProductID: env.ProductID,
						Name: p.String(keyName), Description: p.OptionalString(keyDescription),
						Metadata: metadata, LicenseTemplateID: p.OptionalString("license_template_id"),
					})
					if createErr != nil {
						return nil, createErr
					}
					return organizationOutput(created), nil
				}
				result, err := s.Organizations.CreateWithMember(ctx, organization.CreateOrganizationWithMemberInput{
					TenantID: env.TenantID, ProductID: env.ProductID,
					Name: p.String(keyName), Description: p.OptionalString(keyDescription),
					ProductUserID: owner, RoleID: p.String("owner_role_id"),
					Metadata: metadata, LicenseTemplateID: p.OptionalString("license_template_id"),
				})
				if err != nil {
					return nil, err
				}
				return organizationOutput(result.Organization), nil
			},
			emits: func(params map[string]string) []string {
				emitted := []string{string(events.OrganizationCreated)}
				if strings.TrimSpace(params["owner_product_user_id"]) != "" {
					emitted = append(emitted, string(events.MembershipCreated))
				}
				if strings.TrimSpace(params["license_template_id"]) != "" {
					emitted = append(emitted, string(events.OrganizationLicenseUpdated))
				}
				return emitted
			},
		},
		{
			spec: ActionSpec{
				Type:        ActionOrganizationUpdate,
				Group:       GroupOrganizations,
				Name:        "Update organization",
				Writes:      true,
				MayEmit:     []string{string(events.OrganizationUpdated)},
				Description: "Renames an organization, or merges keys into its metadata. A field left empty keeps its value.",
				Params: []ParamSpec{
					organizationIDParam,
					{Name: keyName, Label: labelName, Type: ParamText},
					{Name: keyDescription, Label: labelDescription, Type: ParamText},
					{Name: keyMetadata, Label: "Metadata to merge", Type: ParamJSON,
						Description: "JSON object merged key by key into the stored metadata."},
				},
				Outputs: organizationOutputs,
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				organizationID := p.String(keyOrganizationID)
				current, err := s.Organizations.Find(ctx, organization.FindOrganizationInput{
					ProductID: env.ProductID, OrganizationID: organizationID,
				})
				if err != nil {
					return nil, err
				}
				patch, _, err := p.Object(keyMetadata)
				if err != nil {
					return nil, err
				}
				metadata := decodeMetadata(current.MetadataJSON)
				maps.Copy(metadata, patch)
				name := current.Name
				if value := p.String(keyName); value != "" {
					name = value
				}
				description := current.Description
				if value := p.OptionalString(keyDescription); value != nil {
					description = value
				}
				updated, err := s.Organizations.Update(ctx, organization.UpdateOrganizationInput{
					ProductID: env.ProductID, OrganizationID: organizationID,
					Name: &name, Description: description, Metadata: metadata,
				})
				if err != nil {
					return nil, err
				}
				return organizationOutput(updated), nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionWorkspaceCreate, Group: GroupWorkspaces, Name: "Create workspace", Writes: true,
				MayEmit:     []string{string(events.WorkspaceCreated)},
				Description: "Creates a workspace inside an organization.",
				Params: []ParamSpec{
					organizationIDParam,
					{Name: keyName, Label: labelName, Type: ParamText, Required: true},
					{Name: keyDescription, Label: labelDescription, Type: ParamText},
				},
				Outputs: []OutputSpec{
					{Name: "workspace_id", Description: "Identifier of the new workspace."},
					{Name: keyName, Description: "Name of the new workspace."},
				},
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				created, err := s.Workspaces.Create(ctx, workspace.CreateWorkspaceInput{
					ProductID: env.ProductID, OrganizationID: p.String(keyOrganizationID),
					Name: p.String(keyName), Description: p.OptionalString(keyDescription),
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{"workspace_id": created.ID, keyName: created.Name}, nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionMemberAdd, Group: GroupMembers, Name: "Add member", Writes: true,
				MayEmit:     []string{string(events.MembershipCreated)},
				Description: "Makes a product user a member of an organization with a role.",
				Params:      []ParamSpec{organizationIDParam, productUserIDParam, roleIDParam},
				Outputs:     membershipOutputs,
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				added, err := s.Memberships.AddMember(ctx, organization.AddMemberInput{
					ProductID: env.ProductID, OrganizationID: p.String(keyOrganizationID),
					ProductUserID: p.String(keyProductUserID), RoleID: p.String(keyRoleID),
				})
				if err != nil {
					return nil, err
				}
				return membershipOutput(added), nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionMemberUpdateRole, Group: GroupMembers, Name: "Change member role", Writes: true,
				MayEmit:     []string{string(events.MembershipUpdated)},
				Description: "Gives a member of an organization another role.",
				Params:      []ParamSpec{organizationIDParam, productUserIDParam, roleIDParam},
				Outputs:     membershipOutputs,
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				updated, err := s.Memberships.UpdateMemberRole(ctx, organization.UpdateMemberRoleInput{
					ProductID: env.ProductID, OrganizationID: p.String(keyOrganizationID),
					ProductUserID: p.String(keyProductUserID), RoleID: p.String(keyRoleID),
				})
				if err != nil {
					return nil, err
				}
				return membershipOutput(updated), nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionMemberRemove, Group: GroupMembers, Name: "Remove member", Writes: true,
				MayEmit:     []string{string(events.MembershipDeleted)},
				Description: "Removes a product user from an organization.",
				Params:      []ParamSpec{organizationIDParam, productUserIDParam},
				Outputs: []OutputSpec{
					{Name: keyOrganizationID, Description: describeOrganizationID},
					{Name: keyProductUserID, Description: "Identifier of the removed member."},
				},
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				input := organization.RemoveMemberInput{
					ProductID: env.ProductID, OrganizationID: p.String(keyOrganizationID),
					ProductUserID: p.String(keyProductUserID),
				}
				if err := s.Memberships.RemoveMember(ctx, input); err != nil {
					return nil, err
				}
				return map[string]any{
					keyOrganizationID: input.OrganizationID, keyProductUserID: input.ProductUserID,
				}, nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionInvitationCreate, Group: GroupMembers, Name: "Invite to organization", Writes: true,
				MayEmit: []string{string(events.OrganizationInvitationCreated)},
				Description: "Creates a pending invitation for an email address. Anchor sends no email: " +
					"follow with a Send email step to tell the person.",
				Params: []ParamSpec{
					organizationIDParam,
					{Name: keyEmail, Label: "Email", Type: ParamEmail, Required: true},
					roleIDParam,
				},
				Outputs: []OutputSpec{
					{Name: "invitation_id", Description: "Identifier of the invitation."},
					{Name: keyEmail, Description: "Invited email address."},
					{Name: "expires_at", Description: "When the invitation expires, RFC 3339."},
				},
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				created, err := s.Invitations.Create(ctx, organizationinvitation.CreateInput{
					ProductID: env.ProductID, OrganizationID: p.String(keyOrganizationID),
					Email: p.String(keyEmail), RoleID: p.String(keyRoleID),
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{
					"invitation_id": created.ID, keyEmail: created.Email,
					"expires_at": created.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
				}, nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionProductUserGet, Group: GroupUsers, Name: "Read product user",
				Description: "Loads a product user, for conditions on their email or for an email step.",
				Params:      []ParamSpec{productUserIDParam},
				Outputs: []OutputSpec{
					{Name: keyProductUserID, Description: "Identifier of the product user."},
					{Name: keyEmail, Description: "Email address."},
					{Name: "email_domain", Description: "Part of the email address after @."},
					{Name: keyName, Description: "Display name."},
					{Name: "external_id", Description: "Identifier at the identity provider."},
					{Name: keyStatus, Description: "ACTIVE or INACTIVE."},
				},
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				found, err := s.ProductUsers.Find(ctx, user.FindProductUserInput{
					ProductID: env.ProductID, ProductUserID: p.String(keyProductUserID),
				})
				if err != nil {
					return nil, err
				}
				output := map[string]any{
					keyProductUserID: found.ID, keyEmail: found.Email, keyName: found.Name,
					"email_domain": emailDomain(found.Email), keyStatus: string(found.Status),
				}
				if found.ExternalID != nil {
					output["external_id"] = *found.ExternalID
				}
				return output, nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionLicenseInstantiate, Group: GroupLicensing, Name: "License organization", Writes: true,
				MayEmit:     []string{string(events.OrganizationLicenseUpdated)},
				Description: "Gives an organization that holds no license its first one, copied from a template.",
				Params: []ParamSpec{
					organizationIDParam,
					{Name: keyTemplateID, Label: labelLicenseTemplate, Type: ParamLicenseTemplate, Required: true},
				},
				Outputs: []OutputSpec{
					{Name: keyLicenseID, Description: "Identifier of the license."},
					{Name: keyTemplateID, Description: "Template the license was copied from."},
				},
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				created, err := s.Licenses.Instantiate(ctx, license.InstantiateLicenseInput{
					TenantID: env.TenantID, ProductID: env.ProductID,
					OrganizationID: p.String(keyOrganizationID), TemplateID: p.String(keyTemplateID),
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{keyLicenseID: created.ID, keyTemplateID: created.TemplateID}, nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionLicenseMigrate, Group: GroupLicensing, Name: "Move to license template", Writes: true,
				MayEmit:     []string{string(events.OrganizationLicenseUpdated)},
				Description: "Migrates one organization onto a license template. Adjusted fields carry forward.",
				Params: []ParamSpec{
					organizationIDParam,
					{Name: keyTemplateID, Label: labelLicenseTemplate, Type: ParamLicenseTemplate, Required: true},
				},
				Outputs: []OutputSpec{
					{Name: "outcome", Description: "CHANGED, UNCHANGED or FAILED."},
					{Name: "previous_template_id", Description: "Template held before, when there was one."},
				},
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				migration, err := s.Migrations.Migrate(ctx, license.MigrateLicensesInput{
					TenantID: env.TenantID, ProductID: env.ProductID, TemplateID: p.String(keyTemplateID),
					OrganizationIDs: []string{p.String(keyOrganizationID)},
				})
				if err != nil {
					return nil, err
				}
				if len(migration.Results) == 0 {
					return nil, fmt.Errorf(
						"the migration reported no result for organization %s",
						p.String(keyOrganizationID),
					)
				}
				result := migration.Results[0]
				if result.Error != nil {
					return nil, result.Error
				}
				output := map[string]any{"outcome": string(result.Outcome)}
				if result.PreviousTemplateID != nil {
					output["previous_template_id"] = *result.PreviousTemplateID
				}
				return output, nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionLicenseAdjust, Group: GroupLicensing, Name: "Adjust license", Writes: true,
				MayEmit:     []string{string(events.OrganizationLicenseUpdated)},
				Description: "Sets license field values on one organization's license without touching its template.",
				Params: []ParamSpec{
					organizationIDParam,
					{Name: "values", Label: "Values", Type: ParamJSON, Required: true,
						Description: `JSON object of license field to value, e.g. {"seats": 25}.`},
				},
				Outputs: []OutputSpec{
					{Name: keyLicenseID, Description: "Identifier of the license."},
					{Name: "adjusted_fields", Description: "Every field now bespoke to the organization."},
				},
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				values, _, err := p.Object("values")
				if err != nil {
					return nil, err
				}
				adjusted, err := s.Licenses.AdjustValues(ctx, license.AdjustLicenseInput{
					TenantID: env.TenantID, ProductID: env.ProductID,
					OrganizationID: p.String(keyOrganizationID), Values: values,
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{
					keyLicenseID: adjusted.ID, "adjusted_fields": strings.Join(adjusted.AdjustedFields, ","),
				}, nil
			},
		},
		{
			spec: ActionSpec{
				Type: ActionEmailSend, Group: GroupEmail, Name: "Send email", Writes: true,
				Description: "Sends a published email template. One run sends a step's email at most once.",
				Params: []ParamSpec{
					{Name: "template_slug", Label: "Email template", Type: ParamEmailTemplate, Required: true},
					{Name: "to", Label: "To", Type: ParamEmail, Required: true},
					{Name: "to_name", Label: "Recipient name", Type: ParamText},
					{Name: "variables", Label: "Variables", Type: ParamJSON,
						Description: "JSON object of template variables. Values may use {{ }} references."},
				},
				Outputs: []OutputSpec{
					{Name: "send_id", Description: "Identifier of the send record."},
					{Name: keyStatus, Description: "Status of the send when the step finished."},
				},
			},
			run: func(ctx context.Context, env Env, p Params) (map[string]any, error) {
				variables, _, err := p.Object("variables")
				if err != nil {
					return nil, err
				}
				dedupeKey := "workflow:" + env.RunID + ":" + env.StepID
				sent, err := s.Email.Send(ctx, email.SendInput{
					TenantID: env.TenantID, ProductID: env.ProductID,
					TemplateSlug: p.OptionalString("template_slug"),
					ToAddress:    p.String("to"), ToName: p.String("to_name"),
					Variables: variables, DedupeKey: &dedupeKey,
				})
				if err != nil {
					return nil, err
				}
				return map[string]any{"send_id": sent.ID, keyStatus: string(sent.Status)}, nil
			},
		},
		httpRequestAction(s.Caller),
		workflowEmitAction(s.CustomEvents),
	}
}

func organizationOutput(found organization.Organization) map[string]any {
	output := map[string]any{
		keyOrganizationID: found.ID,
		keyName:           found.Name,
		keyMetadata:       decodeMetadata(found.MetadataJSON),
	}
	if found.Description != nil {
		output[keyDescription] = *found.Description
	}
	return output
}

func membershipOutput(found organization.Membership) map[string]any {
	return map[string]any{
		keyOrganizationID: found.OrganizationID,
		keyProductUserID:  found.ProductUserID,
		keyRoleID:         found.RoleID,
		"role_name":       found.RoleName,
	}
}

func decodeMetadata(raw json.RawMessage) map[string]any {
	metadata := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &metadata)
	}
	return metadata
}

func emailDomain(address string) string {
	at := strings.LastIndex(address, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(address[at+1:])
}
