import { type WorkflowActionParamResponse, WorkflowParamType } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";
import { DataPanel } from "./DataPanel";
import { EventDataEditor } from "./EventDataEditor";
import { FieldTargetProvider } from "./field-target";
import { demoFields } from "./workflow-fixtures";

const dataParam: WorkflowActionParamResponse = {
	name: "data",
	label: "Data",
	type: WorkflowParamType.JSON,
	required: false,
	literal: false,
};

const typesParam: WorkflowActionParamResponse = {
	name: "data_types",
	label: "Field types",
	type: WorkflowParamType.FIELD_TYPES,
	required: false,
	literal: true,
	types: "data",
};

function StatefulEventData({
	data: initialData,
	types: initialTypes,
	errors = {},
	fieldErrors = {},
}: {
	data: string;
	types: string;
	errors?: Record<string, string>;
	fieldErrors?: Record<string, string>;
}) {
	const [value, setValue] = useState({
		data: initialData,
		types: initialTypes,
	});
	return (
		<FieldTargetProvider>
			<div style={{ width: 460 }}>
				<DataPanel fields={demoFields} />
				<EventDataEditor
					dataParam={dataParam}
					typesParam={typesParam}
					data={value.data}
					types={value.types}
					variables={demoFields}
					errors={errors}
					fieldErrors={fieldErrors}
					onChange={setValue}
				/>
				<output aria-label="Sent">{`${value.data} | ${value.types}`}</output>
			</div>
		</FieldTargetProvider>
	);
}

const meta = {
	title: "Workflow/EventDataEditor",
	component: StatefulEventData,
	args: {
		data: '{"organization_id":"{{event.data.organization_id}}","plan":"pro"}',
		types: '{"organization_id":"organization","plan":"text"}',
	},
} satisfies Meta<typeof StatefulEventData>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Demo: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("group", { name: "Event field 1" }),
		).toBeVisible();
		await expect(
			canvas.getAllByRole("combobox", { name: "Type" })[0],
		).toHaveValue("organization");
	},
};

export const PickingAFieldTypesTheRow: Story = {
	args: { data: "", types: "" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const body = within(canvasElement.ownerDocument.body);
		await userEvent.click(canvas.getByRole("button", { name: "Add field" }));
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Name" }),
			"owner",
		);
		await userEvent.click(
			canvas.getByRole("button", {
				name: "Insert a value into Value of owner",
			}),
		);
		await userEvent.click(
			await body.findByRole("option", { name: /product_user_id/ }),
		);
		await expect(canvas.getByRole("combobox", { name: "Type" })).toHaveValue(
			"product_user",
		);
		await expect(
			canvas.getByRole("status", { name: "Sent" }),
		).toHaveTextContent(
			'{"owner":"{{event.data.product_user_id}}"} | {"owner":"product_user"}',
		);
	},
};

/**
 * Twelve rows, a 60-character name, a duplicate, a name that breaks the key
 * pattern, and a server conflict pinned on the field types.
 */
export const WorstCase: Story = {
	args: {
		data: JSON.stringify(
			Object.fromEntries(
				Array.from({ length: 12 }, (_, index) => [
					index === 0
						? "previous_license_template_identifier_before_the_migration"
						: `field_${index}`,
					index % 2 === 0
						? "{{event.data.organization_id}}"
						: "Northwind Industries Holdings — Europe, Middle East & Africa",
				]),
			),
		),
		types: "",
		errors: {
			data_types:
				'custom.billing.upgraded carries "seats" as text here, but “Provision enterprise onboarding” sends it as number.',
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getAllByRole("group", { name: /Event field/ }),
		).toHaveLength(12);
		await expect(canvas.getByText(/sends it as number/)).toBeVisible();
		const names = canvas.getAllByRole("textbox", { name: "Name" });
		await userEvent.clear(names[2]);
		await userEvent.type(names[2], "field_1");
		await expect(
			canvas.getByText("Another field is already named “field_1”."),
		).toBeVisible();
		await userEvent.clear(names[3]);
		await userEvent.type(names[3], "Bad Name");
		await expect(canvas.getByText(/Use lowercase letters/)).toBeVisible();
	},
};

export const NestedDataStaysJSON: Story = {
	args: { data: '{"seats": 5, "meta": {"plan": "pro"}}', types: "" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("button", { name: "Edit as fields" }),
		).toBeDisabled();
		await expect(canvas.getByRole("textbox", { name: "Data" })).toBeVisible();
	},
};

export const Empty: Story = {
	args: { data: "", types: "" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("The event carries no data yet."),
		).toBeVisible();
	},
};

export const ARemovedRowIsNoLongerATarget: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			canvas.getByRole("textbox", { name: "Value of plan" }),
		);
		await expect(
			canvas.getByText(/Click to insert into Value of plan/),
		).toBeVisible();
		await userEvent.click(
			canvas.getByRole("button", { name: "Remove event field plan" }),
		);
		await expect(
			canvas.getByText("Drag a field onto an input, or click an input first."),
		).toBeVisible();
		await userEvent.click(
			canvas.getByRole("button", {
				name: "email, Email, from Read product user",
			}),
		);
		await expect(
			canvas.getAllByRole("group", { name: /Event field/ }),
		).toHaveLength(1);
	},
};

export const AddingAFieldFocusesItsName: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Add field" }));
		const third = canvas.getByRole("group", { name: "Event field 3" });
		await expect(
			within(third).getByRole("textbox", { name: "Name" }),
		).toHaveFocus();
	},
};

export const AConflictIsPinnedOnItsRow: Story = {
	args: {
		errors: {
			data_types:
				'custom.billing.upgraded carries "plan" as text here, but “Seat sync” sends it as number.',
		},
		fieldErrors: {
			plan: 'custom.billing.upgraded carries "plan" as text here, but “Seat sync” sends it as number.',
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const row = canvas.getByRole("group", { name: "Event field 2" });
		await expect(
			within(row).getByRole("combobox", { name: "Type" }),
		).toHaveAttribute("aria-invalid", "true");
		await expect(canvas.getAllByText(/sends it as number/)).toHaveLength(1);
	},
};

export const RemovingTheFocusedFirstRowDropsTheTarget: Story = {
	args: {
		data: '{"tier":"gold","plan":"pro","seats":"5"}',
		types: '{"tier":"text","plan":"text","seats":"number"}',
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			canvas.getByRole("textbox", { name: "Value of tier" }),
		);
		await userEvent.click(
			canvas.getByRole("button", { name: "Remove event field tier" }),
		);
		await expect(
			canvas.getByText("Drag a field onto an input, or click an input first."),
		).toBeVisible();
		await userEvent.click(
			canvas.getByRole("button", {
				name: "email, Email, from Read product user",
			}),
		);
		await expect(
			canvas.getByRole("textbox", { name: "Value of plan" }),
		).toHaveValue("pro");
		await expect(
			canvas.getByRole("status", { name: "Sent" }),
		).toHaveTextContent(
			'{"plan":"pro","seats":"5"} | {"plan":"text","seats":"number"}',
		);
	},
};
