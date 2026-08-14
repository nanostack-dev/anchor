import {
	LicenseFieldType,
	type LicenseSchemaResponse,
	type LicenseTemplateResponse,
	LicenseTemplateStatus,
} from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, within } from "storybook/test";

import { StoryQuery } from "@/lib/storybook/story-query";

import { LicenseTemplateFormDialog } from "./LicenseTemplateFormDialog";

const TIMESTAMPS = {
	created_at: "2026-08-01T09:00:00Z",
	updated_at: "2026-08-01T09:00:00Z",
};

const SCHEMA: LicenseSchemaResponse = {
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
			name: "sso",
			type: LicenseFieldType.BOOLEAN,
			rules: {},
			...TIMESTAMPS,
		},
	],
	...TIMESTAMPS,
};

const EXISTING_TEMPLATE: LicenseTemplateResponse = {
	id: "ltm_2Nq8xKf3pLmR",
	product_id: "prd_2Nq8xKf3pLmR",
	name: "Pro",
	description: "The tier most customers land on.",
	status: LicenseTemplateStatus.ACTIVE,
	values: { max_flows: 500, sso: true },
	...TIMESTAMPS,
};

const meta = {
	title: "License/LicenseTemplateFormDialog",
	component: LicenseTemplateFormDialog,
	tags: ["autodocs"],
	args: {
		productId: "prd_2Nq8xKf3pLmR",
		schema: SCHEMA,
		trigger: <button type="button">Open</button>,
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
} satisfies Meta<typeof LicenseTemplateFormDialog>;

export default meta;

type Story = StoryObj<typeof meta>;

async function openDialog(canvasElement: HTMLElement, heading: string) {
	const canvas = within(canvasElement);
	await userEvent.click(canvas.getByRole("button", { name: "Open" }));
	return screen.findByRole("heading", { name: heading });
}

/**
 * A create dialog offers one input per field the schema declares, because every
 * declared field is mandatory on a template.
 */
export const CreateOffersEverySchemaField: Story = {
	play: async ({ canvasElement }) => {
		await openDialog(canvasElement, "Create License Template");

		await expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("");
		await expect(
			screen.getByRole("spinbutton", { name: "max_flows" }),
		).toBeInTheDocument();
		await expect(
			screen.getByRole("switch", { name: "sso" }),
		).toBeInTheDocument();
	},
};

/**
 * Edit mode seeds the form from the template on the server, so an operator
 * changing one value does not retype the rest.
 */
export const EditModeSeedsTheExistingTemplate: Story = {
	args: { mode: "edit", existingTemplate: EXISTING_TEMPLATE },
	play: async ({ canvasElement }) => {
		await openDialog(canvasElement, "Edit License Template");

		await expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue(
			"Pro",
		);
		await expect(
			screen.getByDisplayValue("The tier most customers land on."),
		).toBeInTheDocument();
		await expect(
			screen.getByRole("spinbutton", { name: "max_flows" }),
		).toHaveValue(500);
		await expect(
			screen.getByRole("button", { name: "Save Template" }),
		).toBeInTheDocument();
	},
};

/**
 * A template missing any declared value is refused before a request is built,
 * with the error against the field that is missing rather than at the top.
 */
export const IncompleteSubmitNeverReachesTheApi: Story = {
	play: async ({ canvasElement }) => {
		await openDialog(canvasElement, "Create License Template");

		await userEvent.click(
			screen.getByRole("button", { name: "Create Template" }),
		);

		await expect(screen.getByText("Name is required.")).toBeInTheDocument();
		await expect(
			screen.getAllByText("This field is required.").length,
		).toBeGreaterThan(0);
		await expect(
			screen.getByRole("heading", { name: "Create License Template" }),
		).toBeInTheDocument();
	},
};

/**
 * Open purely so the dialog's own framing is reviewable in Storybook. The
 * footer once hung a full step outside the dialog on three sides, and no
 * assertion here could have caught it: the test runner applies none of the
 * utility CSS, reporting this dialog as full width with no padding and no
 * margins, so both the broken and the fixed layout measure identically. This
 * one is checked by eye, in a browser.
 */
export const Framing: Story = {
	play: async ({ canvasElement }) => {
		await openDialog(canvasElement, "Create License Template");

		await expect(
			screen.getByRole("button", { name: "Create Template" }),
		).toBeVisible();
	},
};
