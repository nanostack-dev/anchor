import {
	type WorkflowCondition,
	WorkflowFieldType,
	WorkflowOperator,
} from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";
import { ConditionEditor } from "./ConditionEditor";
import { demoFields, worstCaseFields } from "./workflow-fixtures";
import type { WorkflowVariable } from "./workflow-model";

const verified: WorkflowVariable = {
	path: "steps.product_user.email_verified",
	label: "email_verified",
	source: "Read product user",
	type: WorkflowFieldType.BOOLEAN,
};

const fields = [...demoFields, verified];

function StatefulConditions({
	initial,
	variables,
	width = 400,
}: {
	initial: WorkflowCondition[];
	variables: WorkflowVariable[];
	width?: number;
}) {
	const [conditions, setConditions] = useState(initial);
	return (
		<div style={{ width }}>
			<ConditionEditor
				label="Event condition"
				conditions={conditions}
				variables={variables}
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
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getAllByRole("group", { name: /Event condition \d/ }),
		).toHaveLength(6);
		await expect(
			canvas.getByRole("button", {
				name: /^Event condition 3: value to test: steps\.gone/,
			}),
		).toBeVisible();
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
