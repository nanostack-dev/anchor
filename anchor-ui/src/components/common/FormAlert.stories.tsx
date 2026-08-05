import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { FormAlert } from "./FormAlert";

const meta = {
	title: "Common/FormAlert",
	component: FormAlert,
	tags: ["autodocs"],
	args: {
		message: "That email address is already registered.",
	},
	parameters: {
		layout: "padded",
	},
} satisfies Meta<typeof FormAlert>;

export default meta;

type Story = StoryObj<typeof meta>;

export const ErrorVariant: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Error")).toBeInTheDocument();
		await expect(
			canvas.getByText("That email address is already registered."),
		).toBeInTheDocument();
	},
};

export const Warning: Story = {
	args: {
		variant: "warning",
		message: "This key expires in 3 days.",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Warning")).toBeInTheDocument();
	},
};

export const Info: Story = {
	args: {
		variant: "info",
		message: "Changes apply to new sessions only.",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Information")).toBeInTheDocument();
	},
};

export const Success: Story = {
	args: {
		variant: "success",
		message: "Product created.",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Success")).toBeInTheDocument();
	},
};

/**
 * An explicit title replaces the variant's default heading.
 */
export const CustomTitle: Story = {
	args: {
		title: "Could not save",
		message: "The server rejected the request.",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Could not save")).toBeInTheDocument();
		await expect(canvas.queryByText("Error")).not.toBeInTheDocument();
	},
};

/**
 * No message means no alert at all. Forms render `<FormAlert>` unconditionally
 * and let this branch decide, so an empty message must produce nothing rather
 * than an empty bordered box.
 */
export const NoMessageRendersNothing: Story = {
	args: {
		message: null,
	},
	play: async ({ canvasElement }) => {
		await expect(canvasElement.querySelector("[role='alert']")).toBeNull();
	},
};
