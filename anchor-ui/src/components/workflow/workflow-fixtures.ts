import {
	ProductEventGroupType,
	type WorkflowActionResponse,
	type WorkflowCatalogResponse,
	WorkflowOperator,
	WorkflowParamType,
	type WorkflowResponse,
	type WorkflowRunResponse,
	WorkflowRunStatus,
	WorkflowRunTrigger,
	type WorkflowStep,
	WorkflowStepStatus,
} from "@/client";

const organizationParam = {
	name: "organization_id",
	label: "Organization",
	type: WorkflowParamType.ORGANIZATION,
	required: true,
};

const actions: WorkflowActionResponse[] = [
	{
		type: "workspace.create",
		name: "Create workspace",
		description: "Creates a workspace inside an organization.",
		group: "Workspaces",
		writes: true,
		params: [
			organizationParam,
			{
				name: "name",
				label: "Name",
				type: WorkflowParamType.TEXT,
				required: true,
			},
			{
				name: "description",
				label: "Description",
				type: WorkflowParamType.TEXT,
				required: false,
			},
		],
		outputs: [
			{ name: "workspace_id", description: "Identifier of the new workspace." },
			{ name: "name", description: "Name of the new workspace." },
		],
	},
	{
		type: "organization.update",
		name: "Update organization",
		description: "Renames an organization, or merges keys into its metadata.",
		group: "Organizations",
		writes: true,
		params: [
			organizationParam,
			{
				name: "name",
				label: "Name",
				type: WorkflowParamType.TEXT,
				required: false,
			},
			{
				name: "metadata",
				label: "Metadata to merge",
				type: WorkflowParamType.JSON,
				required: false,
				description: "JSON object merged key by key into the stored metadata.",
			},
		],
		outputs: [
			{
				name: "organization_id",
				description: "Identifier of the organization.",
			},
			{ name: "metadata", description: "Metadata object." },
		],
	},
	{
		type: "product_user.get",
		name: "Read product user",
		description:
			"Loads a product user, for conditions on their email or for an email step.",
		group: "Users",
		writes: false,
		params: [
			{
				name: "product_user_id",
				label: "Product user",
				type: WorkflowParamType.PRODUCT_USER,
				required: true,
			},
		],
		outputs: [
			{ name: "email", description: "Email address." },
			{
				name: "email_domain",
				description: "Part of the email address after @.",
			},
			{ name: "name", description: "Display name." },
		],
	},
	{
		type: "member.add",
		name: "Add member",
		description:
			"Makes a product user a member of an organization with a role.",
		group: "Members",
		writes: true,
		params: [
			organizationParam,
			{
				name: "product_user_id",
				label: "Product user",
				type: WorkflowParamType.PRODUCT_USER,
				required: true,
			},
			{
				name: "role_id",
				label: "Role",
				type: WorkflowParamType.ROLE,
				required: true,
			},
		],
		outputs: [{ name: "role_name", description: "Name of the member's role." }],
	},
	{
		type: "email.send",
		name: "Send email",
		description:
			"Sends a published email template. One run sends a step's email at most once.",
		group: "Email",
		writes: true,
		params: [
			{
				name: "template_slug",
				label: "Email template",
				type: WorkflowParamType.EMAIL_TEMPLATE,
				required: true,
			},
			{
				name: "to",
				label: "To",
				type: WorkflowParamType.EMAIL,
				required: true,
			},
			{
				name: "variables",
				label: "Variables",
				type: WorkflowParamType.JSON,
				required: false,
			},
		],
		outputs: [
			{ name: "send_id", description: "Identifier of the send record." },
		],
	},
];

export const fixtureCatalog: WorkflowCatalogResponse = {
	operators: Object.values(WorkflowOperator),
	triggers: [
		{
			type: "organization.created",
			name: "Organization created",
			description: "Emitted when a new organization is created.",
			group_type: ProductEventGroupType.INTERNAL,
			group_name: "Organizations",
			data_fields: ["organization_id"],
		},
		{
			type: "product_user.created",
			name: "Product user created",
			description: "Emitted when a product user is created.",
			group_type: ProductEventGroupType.INTERNAL,
			group_name: "Users",
			data_fields: ["product_user_id"],
		},
	],
	actions,
};

const timestamps = {
	created_at: "2026-10-01T09:00:00Z",
	updated_at: "2026-10-06T09:00:00Z",
};

export const demoWorkflows: WorkflowResponse[] = [
	{
		id: "wf_demo_workspace",
		name: "Default workspace for new organizations",
		enabled: true,
		trigger_event_type: "organization.created",
		definition: {
			conditions: [],
			steps: [
				{
					id: "workspace",
					action: "workspace.create",
					params: { name: "General" },
				},
				{ id: "tag", action: "organization.update", params: {} },
			],
		},
		...timestamps,
	},
	{
		id: "wf_demo_join",
		name: "Auto-join company domain",
		enabled: false,
		trigger_event_type: "product_user.created",
		definition: {
			conditions: [],
			steps: [
				{ id: "user", action: "product_user.get", params: {} },
				{ id: "join", action: "member.add", params: {} },
			],
		},
		...timestamps,
	},
];

const manySteps: WorkflowStep[] = Array.from({ length: 20 }, (_, index) => ({
	id: `step_${index + 1}`,
	action: actions[index % actions.length].type,
	params: {},
}));

export const worstCaseWorkflows: WorkflowResponse[] = [
	{
		id: "wf_3KMUfROxcFzdWhYAe0JEPxw0bHO",
		name: "Provision enterprise onboarding: workspace, license, owner membership and welcome email for EMEA",
		enabled: true,
		trigger_event_type: "organization.created",
		definition: {
			conditions: [
				{
					field: "event.data.organization_id",
					operator: WorkflowOperator.EXISTS,
				},
			],
			steps: manySteps,
		},
		...timestamps,
	},
	{
		id: "wf_3KMUfRrlnGxqRkW3H6Q0T1h2bDs",
		name: "Jo",
		enabled: false,
		trigger_event_type: "clerk.user.created",
		definition: {
			conditions: [
				{ field: "event.type", operator: WorkflowOperator.EQUALS, value: "a" },
				{ field: "event.type", operator: WorkflowOperator.EQUALS, value: "b" },
			],
			steps: [{ id: "archive", action: "workspace.archive", params: {} }],
		},
		...timestamps,
	},
	{
		id: "wf_3KMUfS0Q2ZbNfpI5rjW0g1V9cXa",
		name: "Synchronise Aleksandra Wiśniewska-Kowalczyk's partner-programme organisations",
		enabled: true,
		trigger_event_type: "product_user.created",
		definition: {
			conditions: [],
			steps: [{ id: "user", action: "product_user.get", params: {} }],
		},
		...timestamps,
	},
];

const runBase = {
	event_type: "organization.created",
	trigger: WorkflowRunTrigger.EVENT,
	started_at: "2026-10-07T09:00:00Z",
	finished_at: "2026-10-07T09:00:00.184Z",
};

export const demoRuns: WorkflowRunResponse[] = [
	{
		...runBase,
		id: "wfrun_demo_1",
		workflow_id: "wf_demo_workspace",
		workflow_name: "Default workspace for new organizations",
		event_id: "pevt_demo_1",
		event_data: { organization_id: "org_3KMUHcINbwY4dpJkNiVGyL2F1xH" },
		status: WorkflowRunStatus.SUCCEEDED,
		steps: [
			{
				step_id: "workspace",
				action: "workspace.create",
				status: WorkflowStepStatus.SUCCEEDED,
				params: {
					organization_id: "org_3KMUHcINbwY4dpJkNiVGyL2F1xH",
					name: "General",
				},
				output: {
					workspace_id: "ws_3KMUHdGeEsaM1QqFqpmSt6zHLXt",
					name: "General",
				},
			},
			{
				step_id: "tag",
				action: "organization.update",
				status: WorkflowStepStatus.SUCCEEDED,
				params: { organization_id: "org_3KMUHcINbwY4dpJkNiVGyL2F1xH" },
				output: { organization_id: "org_3KMUHcINbwY4dpJkNiVGyL2F1xH" },
			},
		],
	},
];

const longError =
	'member.add: ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS: This product user is a member of this organization already.; email.send: parameter "to" is empty after resolving its references';

export const worstCaseRuns: WorkflowRunResponse[] = [
	{
		...runBase,
		id: "wfrun_3KMUh1rGroUkVvGJIzWXi5JkQqE",
		workflow_id: "wf_3KMUfROxcFzdWhYAe0JEPxw0bHO",
		workflow_name:
			"Provision enterprise onboarding: workspace, license, owner membership and welcome email for EMEA",
		event_type: "organization.invitation.accepted",
		event_id: "pevt_3KMUjTqAgZjIo4BR4tk8QqPt4Sd",
		event_data: {
			organization_id: "org_3KMUHcINbwY4dpJkNiVGyL2F1xH",
			invitation_id: "oinv_3KMUHbRkgR1Yq8cTaGkQbdukeSG",
		},
		status: WorkflowRunStatus.FAILED,
		error: longError,
		steps: [
			{
				step_id: "read_the_invited_person_before_anything_x",
				action: "product_user.get",
				status: WorkflowStepStatus.SUCCEEDED,
				params: { product_user_id: "pusr_3KMUfTbJeYkbNFZqQIq2T4YXmBc" },
				output: {
					email:
						"bartholomew.fitzgerald@northwind-industries-holdings.example.com",
					email_domain: "northwind-industries-holdings.example.com",
					name: "Aleksandra Wiśniewska-Kowalczyk",
				},
			},
			{
				step_id: "tag",
				action: "organization.update",
				status: WorkflowStepStatus.SUCCEEDED,
				params: {
					organization_id: "org_3KMUHcINbwY4dpJkNiVGyL2F1xH",
					metadata: {
						billing_ref: "cust_9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c",
						onboarding_url:
							"https://example.com/workspaces/acme/projects/q3-launch/docs/9f8e7d6c5b4a?tab=comments&filter=unresolved",
						regions: ["eu-west-1", "eu-central-1", "us-east-1"],
					},
				},
				output: {},
			},
			{
				step_id: "join",
				action: "member.add",
				status: WorkflowStepStatus.FAILED,
				params: {},
				error:
					"ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS: This product user is a member of this organization already.",
			},
			{
				step_id: "welcome",
				action: "email.send",
				status: WorkflowStepStatus.SKIPPED,
			},
			{
				step_id: "archive",
				action: "workspace.archive",
				status: WorkflowStepStatus.SIMULATED,
			},
		],
	},
	{
		...runBase,
		id: "wfrun_3KMUiYcVtmdmZaHHdhGyN1hgwp4",
		workflow_id: "wf_3KMUfRrlnGxqRkW3H6Q0T1h2bDs",
		workflow_name: "Jo",
		event_id: "pevt_3KMUiYcVtmdmZaHHdhGyN1hgwp5",
		event_data: {},
		status: WorkflowRunStatus.SKIPPED,
		started_at: "2025-08-30T09:00:00Z",
		finished_at: "2025-08-30T09:04:31Z",
		steps: [],
	},
	{
		...runBase,
		id: "wfrun_3KMUjS7GpNhHIjvfBVWy5FNjRKW",
		workflow_id: "wf_3KMUfS0Q2ZbNfpI5rjW0g1V9cXa",
		event_id: "pevt_3KMUjS7GpNhHIjvfBVWy5FNjRKX",
		event_data: { product_user_id: "pusr_3KMUfTbJeYkbNFZqQIq2T4YXmBc" },
		status: WorkflowRunStatus.RUNNING,
		finished_at: undefined,
		steps: [],
	},
];

export const longStepId = "read_the_invited_person_before_anything_x".slice(
	0,
	40,
);

export const worstCaseStep: WorkflowStep = {
	id: longStepId,
	name: "Merge the partner programme, billing reference and onboarding link into the organization's metadata",
	action: "organization.update",
	params: {
		organization_id: "{{event.data.organization_id}}",
		name: "Northwind Industries Holdings — Europe, Middle East & Africa partner organisation",
		metadata:
			'{"billing_ref": "cust_9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c", "onboarding_url": "https://example.com/workspaces/acme/projects/q3-launch/docs/9f8e7d6c5b4a?tab=comments&filter=unresolved"}',
	},
	when: [
		{
			field: "steps.read_the_invited_person_before_anything_x.email_domain",
			operator: WorkflowOperator.IN,
			value:
				"northwind-industries-holdings.example.com, sub.department.region.example.co.uk",
		},
	],
	continue_on_error: true,
};
