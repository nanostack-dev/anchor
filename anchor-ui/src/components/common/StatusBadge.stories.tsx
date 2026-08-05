import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { StatusBadge } from "./StatusBadge";

const meta = {
	title: "Common/StatusBadge",
	component: StatusBadge,
	tags: ["autodocs"],
	args: {
		children: "ACTIVE",
	},
} satisfies Meta<typeof StatusBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("ACTIVE")).toBeInTheDocument();
	},
};

/**
 * An explicit `tone` wins over inference. This is the path callers with a known
 * enum should take — inference is only a fallback for free-text status strings.
 */
export const ExplicitTone: Story = {
	args: {
		tone: "destructive",
		children: "Quota exceeded",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Quota exceeded")).toBeInTheDocument();
	},
};

/**
 * Every tone side by side, so a token change is visible in one shot.
 */
export const AllTones: Story = {
	render: () => (
		<div className="flex flex-wrap items-center gap-2">
			<StatusBadge tone="success">Success</StatusBadge>
			<StatusBadge tone="warning">Warning</StatusBadge>
			<StatusBadge tone="destructive">Destructive</StatusBadge>
			<StatusBadge tone="info">Info</StatusBadge>
			<StatusBadge tone="neutral">Neutral</StatusBadge>
		</div>
	),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		for (const tone of [
			"Success",
			"Warning",
			"Destructive",
			"Info",
			"Neutral",
		]) {
			await expect(canvas.getByText(tone)).toBeInTheDocument();
		}
	},
};

/**
 * Tone inference from the label text — the reason a raw API status can be
 * dropped straight into a table cell without a lookup table at the call site.
 */
export const InferredFromLabel: Story = {
	render: () => (
		<div className="flex flex-wrap items-center gap-2">
			<StatusBadge>ACTIVE</StatusBadge>
			<StatusBadge>PENDING</StatusBadge>
			<StatusBadge>REVOKED</StatusBadge>
			<StatusBadge>UNKNOWN</StatusBadge>
			<StatusBadge>SOMETHING_NEW</StatusBadge>
		</div>
	),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("ACTIVE")).toBeInTheDocument();
		await expect(canvas.getByText("SOMETHING_NEW")).toBeInTheDocument();
	},
};
