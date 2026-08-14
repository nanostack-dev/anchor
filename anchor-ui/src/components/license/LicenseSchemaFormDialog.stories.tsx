import { LicenseFieldType, type LicenseSchemaResponse } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, within } from "storybook/test";

import { StoryQuery } from "@/lib/storybook/story-query";

import { LicenseSchemaFormDialog } from "./LicenseSchemaFormDialog";

const TIMESTAMPS = {
	created_at: "2026-08-01T09:00:00Z",
	updated_at: "2026-08-01T09:00:00Z",
};

const EXISTING_SCHEMA: LicenseSchemaResponse = {
	id: "lsc_2Nq8xKf3pLmR",
	product_id: "prd_2Nq8xKf3pLmR",
	description: "What an Echopoint organization license may carry.",
	fields: [
		{
			id: "lfd_2Nq8xKf3pLmA",
			name: "max_flows",
			type: LicenseFieldType.LIMIT,
			description: "Concurrent flows an organization may run",
			rules: { min: 0, max: 500 },
			...TIMESTAMPS,
		},
		{
			id: "lfd_2Nq8xKf3pLmB",
			name: "tier",
			type: LicenseFieldType.ENUM,
			rules: { values: ["free", "pro", "enterprise"] },
			...TIMESTAMPS,
		},
	],
	...TIMESTAMPS,
};

const meta = {
	title: "License/LicenseSchemaFormDialog",
	component: LicenseSchemaFormDialog,
	tags: ["autodocs"],
	args: {
		productId: "prd_2Nq8xKf3pLmR",
		trigger: <button type="button">Open</button>,
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
 * A fresh create dialog starts with exactly one blank field, already open for
 * typing — an operator declaring a schema for the first time is never shown a
 * schema with zero fields, which the API would technically accept but nobody
 * wants.
 *
 * How fields are authored is covered by `License/LicenseSchemaEditor`; these
 * stories cover what the dialog itself owns.
 */
export const StartsWithOneBlankField: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Open" }));

		await screen.findByRole("heading", { name: "Create License Schema" });
		await expect(screen.getByText("1 field")).toBeInTheDocument();
		await expect(screen.getByPlaceholderText("max_flows")).toBeInTheDocument();
	},
};

/**
 * Edit mode seeds the draft from the schema on the server, collapsed — the
 * reason to open an existing schema is to read it before changing one field.
 */
export const EditModeSeedsTheExistingSchema: Story = {
	args: { mode: "edit", existingSchema: EXISTING_SCHEMA },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Open" }));

		await screen.findByRole("heading", { name: "Edit License Schema" });
		await expect(screen.getByText("2 fields")).toBeInTheDocument();
		await expect(
			screen.getByRole("button", { name: /^max_flows/ }),
		).toHaveAttribute("aria-expanded", "false");
		await expect(
			screen.getByDisplayValue(
				"What an Echopoint organization license may carry.",
			),
		).toBeInTheDocument();
	},
};

/**
 * Submitting an incomplete draft surfaces the error against the offending row
 * and leaves the dialog open. Nothing here touches the network: client-side
 * validation blocks the mutation before a request is ever built.
 */
export const InvalidSubmitNeverReachesTheApi: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Open" }));
		await screen.findByRole("heading", { name: "Create License Schema" });

		await userEvent.click(
			screen.getByRole("button", { name: "Create Schema" }),
		);

		await expect(screen.getByText("Name is required.")).toBeInTheDocument();
		await expect(
			screen.getByRole("heading", { name: "Create License Schema" }),
		).toBeInTheDocument();
	},
};

/**
 * Text mode can hold source the parser cannot read. Submitting then would send
 * whatever last parsed rather than what the operator is looking at, so the
 * submit is refused until the source reads.
 */
export const UnreadableSourceBlocksSubmit: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Open" }));
		await screen.findByRole("heading", { name: "Create License Schema" });

		await userEvent.click(screen.getByRole("button", { name: "Text" }));
		const editor = screen.getByLabelText("Fields");
		await userEvent.click(editor);
		await userEvent.paste("max_flows: integer 0..100");

		await expect(
			screen.getByText(/`integer` is not a field type/),
		).toBeInTheDocument();
		await expect(
			screen.getByRole("button", { name: "Create Schema" }),
		).toBeDisabled();
	},
};
