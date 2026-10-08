import {
	type WorkflowCondition,
	WorkflowFieldType,
	WorkflowOperator,
	WorkflowParamType,
} from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";
import { ConditionEditor } from "./ConditionEditor";
import type { WorkflowResources } from "./useWorkflowResources";
import { demoFields, worstCaseFields } from "./workflow-fixtures";
import type { WorkflowVariable } from "./workflow-model";

const verified: WorkflowVariable = {
	path: "steps.product_user.email_verified",
	label: "email_verified",
	source: "Read product user",
	type: WorkflowFieldType.BOOLEAN,
};

const metadata: WorkflowVariable = {
	path: "steps.product_user.metadata",
	label: "metadata",
	source: "Read product user",
	type: WorkflowFieldType.JSON,
};

const role: WorkflowVariable = {
	path: "steps.role.role_id",
	label: "role_id",
	source: "Create role",
	type: WorkflowFieldType.ROLE,
};

const fields = [...demoFields, verified, metadata];

function StatefulConditions({
	initial,
	variables,
	resources,
	width = 400,
}: {
	initial: WorkflowCondition[];
	variables: WorkflowVariable[];
	resources?: WorkflowResources;
	width?: number;
}) {
	const [conditions, setConditions] = useState(initial);
	return (
		<div style={{ width }}>
			<ConditionEditor
				label="Event condition"
				conditions={conditions}
				variables={variables}
				resources={resources}
				onChange={setConditions}
				emptyLabel="Every event starts a run."
			/>
			<output aria-label="Conditions">{JSON.stringify(conditions)}</output>
		</div>
	);
}

const meta = {
	title: "Workflow/ConditionEditor",
	component: StatefulConditions,
	args: {
		variables: fields,
		initial: [
			{
				field: "steps.product_user.email",
				operator: WorkflowOperator.ENDS_WITH,
				value: "@acme.com",
			},
		],
	},
} satisfies Meta<typeof StatefulConditions>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Demo: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("button", {
				name: "Event condition 1: value to test: email from Read product user, Email",
			}),
		).toBeVisible();
		await expect(
			canvas.getByRole("combobox", { name: "Event condition 1: comparison" }),
		).toHaveValue("ends_with");
	},
};

export const AnIdentifierOffersOnlyEqualityChecks: Story = {
	args: {
		initial: [
			{
				field: "event.data.organization_id",
				operator: WorkflowOperator.EQUALS,
				value: "",
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const comparison = canvas.getByRole("combobox", {
			name: "Event condition 1: comparison",
		});
		await expect(within(comparison).getAllByRole("option")).toHaveLength(5);
		await expect(
			within(comparison).queryByRole("option", { name: "contains" }),
		).toBeNull();
		await expect(
			canvas.getByRole("textbox", { name: "Event condition 1: compared with" }),
		).toHaveAttribute("placeholder", "Organization ID or a field");
	},
};

export const PickingAYesOrNoFieldAsksYesOrNo: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const body = within(canvasElement.ownerDocument.body);
		await userEvent.click(
			canvas.getByRole("button", { name: /^Event condition 1: value to test/ }),
		);
		await userEvent.click(
			await body.findByRole("option", { name: /^email_verified/ }),
		);
		await expect(
			canvas.getByRole("combobox", { name: "Event condition 1: comparison" }),
		).toHaveValue("equals");
		await userEvent.selectOptions(
			canvas.getByRole("combobox", {
				name: "Event condition 1: compared with",
			}),
			"Yes",
		);
		await expect(
			canvas.getByRole("status", { name: "Conditions" }),
		).toHaveTextContent(
			'[{"field":"steps.product_user.email_verified","operator":"equals","value":"true"}]',
		);
	},
};

export const ACustomPathCanBeTyped: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const body = within(canvasElement.ownerDocument.body);
		await userEvent.click(
			canvas.getByRole("button", { name: /^Event condition 1: value to test/ }),
		);
		await userEvent.type(
			await body.findByPlaceholderText("Search or type a path"),
			"steps.product_user.metadata.plan",
		);
		await userEvent.click(
			await body.findByRole("option", {
				name: /Use steps\.product_user\.metadata\.plan/,
			}),
		);
		await expect(
			canvas.getByRole("button", {
				name: "Event condition 1: value to test: steps.product_user.metadata.plan, a custom path",
			}),
		).toBeVisible();
	},
};

/**
 * Six conditions in a 320px column: a 60-character field name, a source named
 * by a 100-character step name, an unknown path and a value on every row.
 */
export const WorstCase: Story = {
	args: {
		width: 320,
		variables: worstCaseFields,
		initial: [
			{
				field: worstCaseFields[0].path,
				operator: WorkflowOperator.EQUALS,
				value: "x",
			},
			{
				field: worstCaseFields[1].path,
				operator: WorkflowOperator.IN,
				value: "a, b, c, d, e, f, g, h",
			},
			{
				field: "steps.gone.metadata.plan_with_a_very_long_name_x",
				operator: WorkflowOperator.CONTAINS,
				value: "pro",
			},
			{
				field: worstCaseFields[4].path,
				operator: WorkflowOperator.EXISTS,
				value: "",
			},
			{
				field: worstCaseFields[5].path,
				operator: WorkflowOperator.EQUALS,
				value: "{{event.data.organization_id}}",
			},
			{
				field: worstCaseFields[6].path,
				operator: WorkflowOperator.NOT_EQUALS,
				value: "25",
			},
		],
	},
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getAllByRole("group", { name: /Event condition \d/ }),
		).toHaveLength(6);
		for (const [index, condition] of args.initial.entries()) {
			await expect(
				canvas.getByRole("combobox", {
					name: `Event condition ${index + 1}: comparison`,
				}),
			).toHaveValue(condition.operator);
		}
		await expect(
			canvas.getByRole("button", {
				name: "Event condition 3: value to test: steps.gone.metadata.plan_with_a_very_long_name_x, not available here",
			}),
		).toBeVisible();
		await expect(
			canvas.getByText(
				"JSON fields cannot use “does not equal”. Pick another comparison.",
			),
		).toBeVisible();
	},
};

/**
 * Saved before conditions were typed, or before the trigger changed: an
 * operator its field no longer offers, a yes-or-no value in capitals and a
 * field the trigger dropped. Each shows what is saved, flagged.
 */
export const ASavedConditionThatNoLongerFits: Story = {
	args: {
		initial: [
			{
				field: "event.data.organization_id",
				operator: WorkflowOperator.CONTAINS,
				value: "org_",
			},
			{
				field: verified.path,
				operator: WorkflowOperator.EQUALS,
				value: "TRUE",
			},
			{
				field: "event.data.removed_field",
				operator: WorkflowOperator.EQUALS,
				value: "x",
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const comparison = canvas.getByRole("combobox", {
			name: "Event condition 1: comparison",
		});
		await expect(comparison).toHaveValue("contains");
		await expect(comparison).toHaveAccessibleDescription(
			"Organization ID fields cannot use “contains”. Pick another comparison.",
		);
		await expect(
			within(comparison).getByRole("option", {
				name: "contains (does not fit)",
			}),
		).toBeInTheDocument();
		await expect(
			canvas.getByText(
				"Organization ID fields cannot use “contains”. Pick another comparison.",
			),
		).toBeVisible();
		await userEvent.selectOptions(comparison, "equals");
		await expect(
			within(comparison).queryByRole("option", { name: /does not fit/ }),
		).toBeNull();
		await expect(canvas.queryByText(/fields cannot use/)).toBeNull();

		await expect(
			canvas.getByRole("combobox", {
				name: "Event condition 2: compared with",
			}),
		).toHaveDisplayValue("Yes");
		await expect(
			canvas.getByRole("button", {
				name: "Event condition 3: value to test: event.data.removed_field, not available here",
			}),
		).toBeVisible();
		const missing = canvas.getByRole("button", {
			name: "Event condition 3: value to test: event.data.removed_field, not available here",
		});
		await expect(missing).toHaveAccessibleDescription(
			"“event.data.removed_field” is not something this condition can read here. Pick a field.",
		);
		await userEvent.click(missing);
		await userEvent.click(
			await within(canvasElement.ownerDocument.body).findByRole("option", {
				name: /^email_domain/,
			}),
		);
		await expect(
			canvas.getByRole("textbox", { name: "Event condition 3: compared with" }),
		).toHaveValue("x");
	},
};

/** Tab to the field, Enter, type and Enter: focus comes back to the field. */
export const PickingAFieldByKeyboard: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const body = within(canvasElement.ownerDocument.body);
		await userEvent.tab();
		const trigger = canvas.getByRole("button", {
			name: /^Event condition 1: value to test/,
		});
		await expect(trigger).toHaveFocus();
		await userEvent.keyboard("{Enter}");
		const search = await body.findByPlaceholderText("Search or type a path");
		await expect(search).toHaveFocus();
		await expect(
			await body.findByRole("option", { name: /^email\b/ }),
		).toHaveAttribute("data-checked", "true");
		await userEvent.type(search, "email_domain");
		await userEvent.keyboard("{Enter}");
		const picked = canvas.getByRole("button", {
			name: "Event condition 1: value to test: email_domain from Read product user, Text",
		});
		await expect(picked).toHaveFocus();
		await expect(
			canvas.getByRole("status", { name: "Conditions" }),
		).toHaveTextContent(
			'[{"field":"steps.product_user.email_domain","operator":"ends_with","value":""}]',
		);
	},
};

/** A role field suggests the product's roles, except in a list. */
export const ARoleIsSuggested: Story = {
	args: {
		variables: [...fields, role],
		resources: {
			[WorkflowParamType.ROLE]: [{ value: "role_admin", label: "Admin" }],
		},
		initial: [
			{ field: role.path, operator: WorkflowOperator.EQUALS, value: "" },
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const value = canvas.getByRole("combobox", {
			name: "Event condition 1: compared with",
		});
		const listId = value.getAttribute("list");
		await expect(listId).toBeTruthy();
		await expect(
			canvasElement.ownerDocument.getElementById(listId ?? "")?.textContent,
		).toContain("Admin");
		await userEvent.selectOptions(
			canvas.getByRole("combobox", { name: "Event condition 1: comparison" }),
			"in",
		);
		await expect(
			canvas.getByRole("textbox", { name: "Event condition 1: compared with" }),
		).not.toHaveAttribute("list");
	},
};

export const Empty: Story = {
	args: { initial: [] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Every event starts a run.")).toBeVisible();
		await userEvent.click(
			canvas.getByRole("button", { name: "Add condition" }),
		);
		await expect(
			canvas.getByRole("button", {
				name: "Event condition 1: value to test: organization_id from Event, Organization ID",
			}),
		).toBeVisible();
	},
};
