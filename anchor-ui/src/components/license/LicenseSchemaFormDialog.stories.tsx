import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, within } from "storybook/test";

import { StoryQuery } from "@/lib/storybook/story-query";

import { LicenseSchemaFormDialog } from "./LicenseSchemaFormDialog";

const meta = {
	title: "License/LicenseSchemaFormDialog",
	component: LicenseSchemaFormDialog,
	tags: ["autodocs"],
	args: {
		productId: "prd_2Nq8xKf3pLmR",
		trigger: <button type="button">Create Schema</button>,
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
} satisfies Meta<typeof LicenseSchemaFormDialog>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * A fresh create dialog starts with exactly one blank field row — an operator
 * declaring a schema for the first time is never shown a schema with zero
 * fields, which the API would technically accept but nobody wants.
 */
export const StartsWithOneBlankField: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			canvas.getByRole("button", { name: "Create Schema" }),
		);

		await screen.findByRole("heading", { name: "Create License Schema" });
		await expect(screen.getAllByPlaceholderText("max_flows")).toHaveLength(1);
	},
};

/**
 * "Add field" grows the list, and each row's own remove button shrinks it
 * back down — the dynamic array editor round-trips.
 */
export const AddAndRemoveField: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			canvas.getByRole("button", { name: "Create Schema" }),
		);
		await screen.findByRole("heading", { name: "Create License Schema" });

		await userEvent.click(screen.getByRole("button", { name: "Add field" }));
		await expect(screen.getAllByPlaceholderText("max_flows")).toHaveLength(2);

		await userEvent.click(
			screen.getAllByRole("button", { name: "Remove field" })[0],
		);
		await expect(screen.getAllByPlaceholderText("max_flows")).toHaveLength(1);
	},
};

/**
 * Submitting with a blank name surfaces the error against that row, not as a
 * lone toast — the acceptance criterion is "against the offending field".
 * Nothing here touches the network: client-side validation blocks the
 * mutation before a request is ever built.
 */
export const BlankNameIsRejectedAgainstItsRow: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			canvas.getByRole("button", { name: "Create Schema" }),
		);
		await screen.findByRole("heading", { name: "Create License Schema" });

		await userEvent.click(
			screen.getByRole("button", { name: "Create Schema" }),
		);

		await expect(screen.getByText("Name is required.")).toBeInTheDocument();
		// The dialog stays open — an invalid submit never fires the mutation.
		await expect(
			screen.getByRole("heading", { name: "Create License Schema" }),
		).toBeInTheDocument();
	},
};

/**
 * Two rows sharing a name both get flagged: the API keys errors by field
 * name, so a form that only highlighted one row would leave the other's
 * problem invisible until the request round-trips.
 */
export const DuplicateNamesAreBothFlagged: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			canvas.getByRole("button", { name: "Create Schema" }),
		);
		await screen.findByRole("heading", { name: "Create License Schema" });

		await userEvent.click(screen.getByRole("button", { name: "Add field" }));
		const nameInputs = screen.getAllByPlaceholderText("max_flows");
		await userEvent.type(nameInputs[0], "max_flows");
		await userEvent.type(nameInputs[1], "max_flows");

		await userEvent.click(
			screen.getByRole("button", { name: "Create Schema" }),
		);

		await expect(
			screen.getAllByText("This field name is used more than once."),
		).toHaveLength(2);
	},
};

/**
 * Switching a field's type to Enum swaps the rule editor to the "allowed
 * values" input — the rule inputs shown are conditioned on the type selected
 * in the same row, not a fixed set.
 */
export const TypeChangeSwapsRuleInputs: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			canvas.getByRole("button", { name: "Create Schema" }),
		);
		await screen.findByRole("heading", { name: "Create License Schema" });

		// A new row defaults to String, whose rule editor shows a pattern and
		// length bounds — not the numeric Min/Max a Limit or Number field gets.
		await expect(
			screen.getByLabelText("Pattern (regular expression)"),
		).toBeInTheDocument();

		await userEvent.click(screen.getByRole("combobox"));
		await userEvent.click(await screen.findByRole("option", { name: "Enum" }));

		await expect(
			screen.getByLabelText("Allowed values (comma-separated)"),
		).toBeInTheDocument();
		await expect(
			screen.queryByLabelText("Pattern (regular expression)"),
		).not.toBeInTheDocument();
	},
};
