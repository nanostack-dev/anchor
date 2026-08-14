import { LicenseFieldType, type LicenseTemplateValues } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, screen, userEvent, within } from "storybook/test";

import { LicenseValueFields } from "./LicenseValueFields";

const FIELDS = [
	{
		id: "fld_limit",
		name: "max_flows",
		type: LicenseFieldType.LIMIT,
		description: "Concurrent flows allowed",
		rules: { max: 1000 },
		created_at: "2026-07-01T00:00:00Z",
		updated_at: "2026-07-01T00:00:00Z",
	},
	{
		id: "fld_bool",
		name: "sso",
		type: LicenseFieldType.BOOLEAN,
		description: "Single sign-on enabled",
		rules: {},
		created_at: "2026-07-01T00:00:00Z",
		updated_at: "2026-07-01T00:00:00Z",
	},
	{
		id: "fld_enum",
		name: "support_tier",
		type: LicenseFieldType.ENUM,
		description: "",
		rules: { values: ["standard", "priority"] },
		created_at: "2026-07-01T00:00:00Z",
		updated_at: "2026-07-01T00:00:00Z",
	},
];

const meta = {
	title: "License/LicenseValueFields",
	component: LicenseValueFields,
	tags: ["autodocs"],
	args: {
		fields: FIELDS,
		values: { max_flows: 500, sso: true, support_tier: "priority" },
	},
} satisfies Meta<typeof LicenseValueFields>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * Without an `onChange`, every field type renders as plain text — this is
 * the shape the organization license view and the template detail view use,
 * and a boolean must read as "Yes"/"No" rather than a raw `true`.
 */
export const ReadOnlyFormatsEveryType: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("500")).toBeInTheDocument();
		await expect(canvas.getByText("Yes")).toBeInTheDocument();
		await expect(canvas.getByText("priority")).toBeInTheDocument();
		await expect(canvas.queryByRole("switch")).not.toBeInTheDocument();
	},
};

/**
 * With an `onChange`, each field type gets the input that matches it: a
 * number box for a limit, a switch for a boolean, a select constrained to the
 * declared values for an enum.
 */
export const EditableRendersTypedInputs: Story = {
	args: {
		onChange: () => {},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByRole("spinbutton")).toHaveValue(500);
		await expect(canvas.getByRole("switch")).toBeChecked();
		await expect(canvas.getByRole("combobox")).toBeInTheDocument();
	},
};

/** Holds its own state so a story can prove a round-trip through `onChange`. */
function StatefulHarness({
	values: initialValues,
}: {
	values: LicenseTemplateValues;
}) {
	const [values, setValues] = useState(initialValues);
	return (
		<LicenseValueFields
			fields={FIELDS}
			values={values}
			onChange={(name, value) =>
				setValues((prev) => ({ ...prev, [name]: value }))
			}
		/>
	);
}

/**
 * Typing into the limit's number input must round-trip a real `number` back
 * through `onChange`, not the raw string the DOM input holds. A component
 * that delivered a string would fail the controlled-input's own
 * `typeof value === "number"` check on every keystroke and the field would
 * never accumulate past one digit.
 */
export const NumberInputRoundTripsANumber: Story = {
	render: () => (
		<StatefulHarness
			values={{ max_flows: undefined, sso: false, support_tier: "standard" }}
		/>
	),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const input = canvas.getByRole("spinbutton");

		await userEvent.type(input, "42");

		await expect(input).toHaveValue(42);
	},
};

/**
 * A required field carrying a validation error renders that error under its
 * own input, matching the acceptance criterion that errors surface against
 * the offending field rather than only as a toast.
 */
export const SurfacesAFieldError: Story = {
	args: {
		onChange: () => {},
		errors: { max_flows: "This field is required." },
	},
	play: async () => {
		await expect(
			screen.getByText("This field is required."),
		).toBeInTheDocument();
	},
};
