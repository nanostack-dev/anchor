import { type WorkflowActionParamResponse, WorkflowParamType } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { DataPanel } from "./DataPanel";
import { ParamInput } from "./ParamInput";
import { FieldTargetProvider } from "./field-target";
import { demoFields, worstCaseFields } from "./workflow-fixtures";
import type { WorkflowVariable } from "./workflow-model";

const organization: WorkflowActionParamResponse = {
	name: "organization_id",
	label: "Organization",
	type: WorkflowParamType.ORGANIZATION,
	required: true,
	literal: false,
};

const name: WorkflowActionParamResponse = {
	name: "name",
	label: "Name",
	type: WorkflowParamType.TEXT,
	required: false,
	literal: false,
};

function StatefulParams({
	params,
	initial,
	fields,
}: {
	params: WorkflowActionParamResponse[];
	initial: Record<string, string>;
	fields: WorkflowVariable[];
}) {
	const [values, setValues] = useState(initial);
	return (
		<FieldTargetProvider>
			<div
				style={{
					width: 420,
					display: "grid",
					gridTemplateColumns: "minmax(0, 1fr)",
					gap: 16,
				}}
			>
				<DataPanel fields={fields} />
				{params.map((param) => (
					<ParamInput
						key={param.name}
						param={param}
						value={values[param.name] ?? ""}
						variables={fields}
						resources={{}}
						onChange={(value) =>
							setValues((current) => ({ ...current, [param.name]: value }))
						}
					/>
				))}
				<output aria-label="Values">{JSON.stringify(values)}</output>
			</div>
		</FieldTargetProvider>
	);
}

const meta = {
	title: "Workflow/ParamInput",
	component: StatefulParams,
	args: { params: [organization, name], initial: {}, fields: demoFields },
} satisfies Meta<typeof StatefulParams>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Demo: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const body = within(canvasElement.ownerDocument.body);
		await userEvent.click(
			canvas.getByRole("button", { name: "Insert a value into Organization" }),
		);
		const fits = await body.findByRole("group", {
			name: "Fits: Organization ID",
		});
		await userEvent.click(
			within(fits).getByRole("option", { name: /organization_id/ }),
		);
		await expect(
			await canvas.findByRole("button", {
				name: "Organization *: organization_id from Event. Edit",
			}),
		).toBeVisible();
		await expect(
			canvas.getByRole("status", { name: "Values" }),
		).toHaveTextContent('"organization_id":"{{event.data.organization_id}}"');
	},
};

export const TypedReferenceCompletes: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const input = canvas.getByRole("textbox", { name: "Name" });
		await userEvent.type(input, "Hi {{{{emai");
		const list = await canvas.findByRole("listbox", {
			name: "Fields for Name",
		});
		await expect(within(list).getAllByRole("option")).toHaveLength(2);
		await userEvent.keyboard("{Enter}");
		await expect(input).toHaveValue("Hi {{steps.product_user.email}}");
	},
};

export const ClickAChipIntoTheLastInput: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("textbox", { name: "Name" }));
		await userEvent.click(
			canvas.getByRole("button", {
				name: "email, Email, from Read product user",
			}),
		);
		await expect(
			await canvas.findByRole("button", {
				name: "Name: email from Read product user. Edit",
			}),
		).toBeVisible();
	},
};

export const Mismatch: Story = {
	args: { initial: { organization_id: "{{event.data.product_user_id}}" } },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText(/Organization expects Organization ID/),
		).toBeVisible();
	},
};

/**
 * Thirty fields from four sources, a 100-character step name as a source and
 * identifiers longer than a chip, with one of them in the pill.
 */
export const WorstCase: Story = {
	args: {
		fields: worstCaseFields,
		initial: {
			name: `{{${worstCaseFields[1].path}}}`,
			organization_id:
				"{{event.data.removed_long_ago_and_not_in_the_catalog_anymore}}",
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("button", {
				name: /^Name: field_1 from Merge the partner/,
			}),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", {
				name: /^Organization \*: event\.data\.removed_long_ago.* from not available here/,
			}),
		).toBeVisible();
		await waitFor(() =>
			expect(
				canvas.getByRole("region", { name: "Data you can use" }),
			).toBeVisible(),
		);
	},
};

export const Empty: Story = {
	args: { fields: [] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("button", { name: "Insert a value into Organization" }),
		).toBeDisabled();
		await expect(
			canvas.queryByRole("region", { name: "Data you can use" }),
		).toBeNull();
	},
};

export const CompletingAWholeReferenceFocusesItsPill: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Name" }),
			"{{{{emai",
		);
		await canvas.findByRole("listbox", { name: "Fields for Name" });
		await userEvent.keyboard("{Enter}");
		await waitFor(() =>
			expect(
				canvas.getByRole("button", {
					name: "Name: email from Read product user. Edit",
				}),
			).toHaveFocus(),
		);
	},
};

export const ClearingAPillFocusesTheInput: Story = {
	args: { initial: { name: "{{steps.product_user.email}}" } },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Clear Name" }));
		await waitFor(() =>
			expect(canvas.getByRole("textbox", { name: "Name" })).toHaveFocus(),
		);
	},
};
