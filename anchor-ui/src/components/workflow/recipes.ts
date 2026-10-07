import { WorkflowOperator } from "@/client";
import type { WorkflowDraft } from "./workflow-model";

export interface WorkflowRecipe {
	id: string;
	title: string;
	summary: string;
	draft: WorkflowDraft;
}

export const workflowRecipes: WorkflowRecipe[] = [
	{
		id: "default-workspace",
		title: "Give every new organization a workspace",
		summary:
			"When an organization is created, create a General workspace and record it in the organization's metadata.",
		draft: {
			name: "Default workspace for new organizations",
			description: "",
			enabled: true,
			trigger_event_type: "organization.created",
			definition: {
				conditions: [],
				steps: [
					{
						id: "workspace",
						name: "Create General workspace",
						action: "workspace.create",
						params: {
							organization_id: "{{event.data.organization_id}}",
							name: "General",
						},
						when: [],
					},
					{
						id: "tag",
						name: "Remember it on the organization",
						action: "organization.update",
						params: {
							organization_id: "{{event.data.organization_id}}",
							metadata:
								'{"default_workspace_id": "{{steps.workspace.workspace_id}}"}',
						},
						when: [],
					},
				],
			},
		},
	},
	{
		id: "domain-auto-join",
		title: "Auto-join users from a company domain",
		summary:
			"When a product user signs up with a company email, add them to that company's organization with a role.",
		draft: {
			name: "Auto-join company domain",
			description: "",
			enabled: false,
			trigger_event_type: "product_user.created",
			definition: {
				conditions: [],
				steps: [
					{
						id: "user",
						name: "Read the new user",
						action: "product_user.get",
						params: { product_user_id: "{{event.data.product_user_id}}" },
						when: [],
					},
					{
						id: "join",
						name: "Add to the company organization",
						action: "member.add",
						params: {
							organization_id: "",
							product_user_id: "{{steps.user.product_user_id}}",
							role_id: "",
						},
						when: [
							{
								field: "steps.user.email_domain",
								operator: WorkflowOperator.EQUALS,
								value: "acme.com",
							},
						],
					},
				],
			},
		},
	},
	{
		id: "personal-organization",
		title: "Create a personal organization at sign-up",
		summary:
			"When a product user is created, create an organization named after them and make them its owner.",
		draft: {
			name: "Personal organization at sign-up",
			description: "",
			enabled: false,
			trigger_event_type: "product_user.created",
			definition: {
				conditions: [],
				steps: [
					{
						id: "user",
						name: "Read the new user",
						action: "product_user.get",
						params: { product_user_id: "{{event.data.product_user_id}}" },
						when: [],
					},
					{
						id: "org",
						name: "Create their organization",
						action: "organization.create",
						params: {
							name: "{{steps.user.name}}'s workspace",
							owner_product_user_id: "{{steps.user.product_user_id}}",
							owner_role_id: "",
						},
						when: [],
					},
				],
			},
		},
	},
	{
		id: "license-new-organizations",
		title: "License new organizations on a template",
		summary:
			"When an organization is created, give it its first license from a template you pick.",
		draft: {
			name: "License new organizations",
			description: "",
			enabled: false,
			trigger_event_type: "organization.created",
			definition: {
				conditions: [],
				steps: [
					{
						id: "license",
						name: "Instantiate the license",
						action: "license.instantiate",
						params: {
							organization_id: "{{event.data.organization_id}}",
							template_id: "",
						},
						when: [],
					},
				],
			},
		},
	},
	{
		id: "welcome-member",
		title: "Welcome every new member by email",
		summary:
			"When a member joins an organization, read who they are and where they joined, then send a welcome email.",
		draft: {
			name: "Welcome new members",
			description: "",
			enabled: false,
			trigger_event_type: "organization.membership.created",
			definition: {
				conditions: [],
				steps: [
					{
						id: "user",
						name: "Read the member",
						action: "product_user.get",
						params: { product_user_id: "{{event.data.product_user_id}}" },
						when: [],
					},
					{
						id: "org",
						name: "Read the organization",
						action: "organization.get",
						params: { organization_id: "{{event.data.organization_id}}" },
						when: [],
					},
					{
						id: "welcome",
						name: "Send the welcome email",
						action: "email.send",
						params: {
							template_slug: "welcome",
							to: "{{steps.user.email}}",
							to_name: "{{steps.user.name}}",
							variables:
								'{"name": "{{steps.user.name}}", "organization": "{{steps.org.name}}"}',
						},
						when: [],
					},
				],
			},
		},
	},
];

export function findRecipe(id: string | undefined): WorkflowRecipe | undefined {
	return workflowRecipes.find((recipe) => recipe.id === id);
}
