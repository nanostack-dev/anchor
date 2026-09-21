import {
	LicenseFieldType,
	type LicenseSchemaResponse,
	type LicenseTemplateResponse,
	LicenseTemplateStatus,
	UsageShape,
	client,
} from "@/client";
import { StoryQuery } from "@/lib/storybook/story-query";
import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { LicenseTemplateEditView } from "./LicenseTemplateEditView";

const TIMESTAMPS = {
	created_at: "2026-08-16T17:27:00Z",
	updated_at: "2026-08-16T17:27:00Z",
};
const SCHEMA: LicenseSchemaResponse = {
	id: "lsc_test",
	product_id: "prd_test",
	...TIMESTAMPS,
	fields: [
		{
			...TIMESTAMPS,
			id: "lfd_duration",
			name: "max_cloud_execution_duration_seconds",
			type: LicenseFieldType.NUMBER,
			description: "Wall-clock ceiling for one cloud execution",
			rules: { min: 60, max: 86400 },
		},
		{
			...TIMESTAMPS,
			id: "lfd_flows",
			name: "max_flows",
			type: LicenseFieldType.LIMIT,
			usage_shape: UsageShape.GAUGE,
			description: "Flows this organization may define",
			rules: { min: 1, max: 100000 },
		},
		{
			...TIMESTAMPS,
			id: "lfd_runs",
			name: "monthly_runs",
			type: LicenseFieldType.LIMIT,
			usage_shape: UsageShape.WINDOWED_COUNTER,
			description: "Flow runs consumed in the current billing period",
			rules: { min: 0, max: 50000000 },
		},
		{
			...TIMESTAMPS,
			id: "lfd_region",
			name: "region",
			type: LicenseFieldType.STRING,
			description: "Where this customer is hosted",
			rules: {},
		},
		{
			...TIMESTAMPS,
			id: "lfd_sso",
			name: "sso",
			type: LicenseFieldType.BOOLEAN,
			description: "SAML single sign-on",
			rules: {},
		},
		{
			...TIMESTAMPS,
			id: "lfd_support",
			name: "support_tier",
			type: LicenseFieldType.ENUM,
			description: "Response-time commitment",
			rules: { values: ["community", "standard", "priority", "dedicated"] },
		},
	],
};
const TEMPLATE: LicenseTemplateResponse = {
	...TIMESTAMPS,
	id: "ltm_test",
	product_id: "prd_test",
	name: "Enterprise",
	description: "Negotiated, with an SLA.",
	status: LicenseTemplateStatus.ACTIVE,
	values: {
		max_cloud_execution_duration_seconds: 21600,
		max_flows: 10000,
		monthly_runs: 2000000,
		region: "eu-west",
		sso: true,
		support_tier: "dedicated",
	},
};

function TemplateHarness({
	initialTemplate = TEMPLATE,
	initialEditing = false,
	create = false,
}: {
	initialTemplate?: LicenseTemplateResponse;
	initialEditing?: boolean;
	create?: boolean;
}) {
	const [template, setTemplate] = useState(
		create ? undefined : initialTemplate,
	);
	const [editing, setEditing] = useState(initialEditing);
	return (
		<LicenseTemplateEditView
			productId="prd_test"
			schema={SCHEMA}
			template={template}
			editing={editing}
			onEdit={() => setEditing(true)}
			onCancel={() => setEditing(false)}
			onSaved={(saved) => {
				setTemplate(saved);
				setEditing(false);
			}}
		/>
	);
}

const meta = {
	title: "License/LicenseTemplateEditView",
	component: TemplateHarness,
	parameters: { layout: "fullscreen" },
	decorators: [
		(Story) => (
			<StoryRouter>
				<StoryQuery>
					<Story />
				</StoryQuery>
			</StoryRouter>
		),
	],
} satisfies Meta<typeof TemplateHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const View: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByRole("heading", { name: "Enterprise", level: 1 }),
		).toBeVisible();
		await expect(canvas.queryByRole("dialog")).not.toBeInTheDocument();
		await expect(
			canvas.getByText("Wall-clock ceiling for one cloud execution"),
		).toBeVisible();
		await expect(canvas.getByText("21600")).toBeVisible();
		await expect(
			canvas.getByRole("link", { name: "All templates" }),
		).toHaveAttribute("href", "/products/licensing/templates");
	},
};
export const Edit: Story = { args: { initialEditing: true } };
export const CancelDiscardsEdits: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			await canvas.findByRole("button", { name: "Edit template" }),
		);
		await expect(canvas.getByRole("textbox", { name: "Name" })).toHaveValue(
			"Enterprise",
		);
		const flows = canvas.getByRole("spinbutton", {
			name: "max_flows",
		});
		await userEvent.clear(flows);
		await userEvent.type(flows, "25000");
		await userEvent.click(canvas.getByRole("button", { name: "Cancel" }));
		await expect(canvas.getByText("10000")).toBeVisible();
		await userEvent.click(
			canvas.getByRole("button", { name: "Edit template" }),
		);
		await expect(
			canvas.getByRole("spinbutton", { name: "max_flows" }),
		).toHaveValue(10000);
	},
};
export const SaveReturnsToUpdatedView: Story = {
	args: { initialEditing: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const previousConfig = client.getConfig();
		const requestBody = fn();
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async (request) => {
				if (!(request instanceof Request)) throw new Error("Expected Request");
				expect(request.method).toBe("PUT");
				expect(new URL(request.url).pathname).toBe(
					"/v1/products/prd_test/licensing/templates/ltm_test",
				);
				const body = await request.json();
				requestBody(body);
				return Response.json({ ...TEMPLATE, ...body });
			},
		});
		try {
			const flows = await canvas.findByRole("spinbutton", {
				name: "max_flows",
			});
			await userEvent.clear(flows);
			await userEvent.type(flows, "25000");
			await userEvent.click(
				canvas.getByRole("button", { name: "Save Template" }),
			);
			await waitFor(() =>
				expect(requestBody).toHaveBeenCalledWith({
					name: TEMPLATE.name,
					description: TEMPLATE.description,
					values: { ...TEMPLATE.values, max_flows: 25000 },
				}),
			);
			await expect(
				await canvas.findByRole("heading", { name: "Enterprise", level: 1 }),
			).toBeVisible();
			await expect(canvas.getByText("25000")).toBeVisible();
			await expect(requestBody).toHaveBeenCalledTimes(1);
		} finally {
			client.setConfig(previousConfig);
		}
	},
};
export const FailedSaveKeepsDraft: Story = {
	args: { initialEditing: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const previousConfig = client.getConfig();
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async () =>
				Response.json(
					{
						errors: [
							{
								code: "INVALID_VALUES",
								field: "max_flows",
								message: "The flow limit is too high.",
							},
						],
					},
					{ status: 400 },
				),
		});
		try {
			const flows = await canvas.findByRole("spinbutton", {
				name: "max_flows",
			});
			await userEvent.clear(flows);
			await userEvent.type(flows, "25000");
			await userEvent.click(
				canvas.getByRole("button", { name: "Save Template" }),
			);
			await waitFor(() =>
				expect(flows).toHaveAttribute("aria-invalid", "true"),
			);
			await expect(flows).toHaveValue(25000);
			await expect(
				canvas.getByRole("heading", { name: "Edit Enterprise" }),
			).toBeVisible();
		} finally {
			client.setConfig(previousConfig);
		}
	},
};
export const ArchivedCannotEdit: Story = {
	args: {
		initialTemplate: { ...TEMPLATE, status: LicenseTemplateStatus.ARCHIVED },
		initialEditing: true,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByText(
				"This template is archived and can no longer be edited.",
			),
		).toBeVisible();
		await expect(
			canvas.queryByRole("button", { name: "Edit template" }),
		).not.toBeInTheDocument();
		await expect(
			canvas.queryByRole("button", { name: "Save Template" }),
		).not.toBeInTheDocument();
		await expect(canvas.queryByRole("textbox")).not.toBeInTheDocument();
	},
};
export const IncompleteCreateIsRejected: Story = {
	args: { create: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			await canvas.findByRole("button", { name: "Create Template" }),
		);
		await expect(canvas.getByText("Name is required.")).toBeVisible();
		await expect(
			canvas.getAllByText("This field is required.").length,
		).toBeGreaterThan(0);
	},
};
export const CreateSendsAllValues: Story = {
	args: { create: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const previousConfig = client.getConfig();
		const requestBody = fn();
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async (request) => {
				if (!(request instanceof Request)) throw new Error("Expected Request");
				expect(request.method).toBe("POST");
				expect(new URL(request.url).pathname).toBe(
					"/v1/products/prd_test/licensing/templates",
				);
				const body = await request.json();
				requestBody(body);
				return Response.json({ ...TEMPLATE, ...body });
			},
		});
		try {
			await userEvent.type(
				await canvas.findByRole("textbox", { name: "Name" }),
				"New tier",
			);
			await userEvent.type(
				canvas.getByRole("spinbutton", {
					name: "max_cloud_execution_duration_seconds",
				}),
				"60",
			);
			await userEvent.type(
				canvas.getByRole("spinbutton", { name: "max_flows" }),
				"10",
			);
			await userEvent.type(
				canvas.getByRole("spinbutton", { name: "monthly_runs" }),
				"0",
			);
			await userEvent.type(
				canvas.getByRole("textbox", { name: "region" }),
				"eu-west",
			);
			await userEvent.click(canvas.getByRole("switch", { name: "sso" }));
			await userEvent.click(canvas.getByRole("switch", { name: "sso" }));
			await userEvent.click(
				canvas.getByRole("combobox", { name: "support_tier" }),
			);
			await userEvent.click(
				await within(document.body).findByRole("option", {
					name: "standard",
				}),
			);
			await userEvent.click(
				canvas.getByRole("button", { name: "Create Template" }),
			);
			await waitFor(() =>
				expect(requestBody).toHaveBeenCalledWith({
					name: "New tier",
					description: "",
					values: {
						max_cloud_execution_duration_seconds: 60,
						max_flows: 10,
						monthly_runs: 0,
						region: "eu-west",
						sso: false,
						support_tier: "standard",
					},
				}),
			);
			await expect(
				await canvas.findByRole("heading", { name: "New tier", level: 1 }),
			).toBeVisible();
		} finally {
			client.setConfig(previousConfig);
		}
	},
};
