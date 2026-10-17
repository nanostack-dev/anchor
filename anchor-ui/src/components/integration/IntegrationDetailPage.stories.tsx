import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import {
	type IntegrationAuditEntry,
	IntegrationDetailPage,
} from "./IntegrationDetailPage";
const meta = {
	title: "Integrations/Detail",
	component: IntegrationDetailPage,
	decorators: [
		(Story) => (
			<StoryRouter>
				<Story />
			</StoryRouter>
		),
	],
	args: {
		title: "Clerk Authentication",
		description: "Configure identity events for your product.",
		backLink: null,
		summary: [{ label: "Provider", value: "Clerk" }],
		children: null,
		auditEntries: [],
	},
} satisfies Meta<typeof IntegrationDetailPage>;
export default meta;
type Story = StoryObj<typeof meta>;
export const EmptyAudit: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("heading", { name: "Clerk Authentication" }),
		).toBeVisible();
		await expect(canvas.getByText("No activity recorded yet.")).toBeVisible();
	},
};
export const RecentActivity: Story = {
	args: {
		auditEntries: [
			{
				id: "older",
				title: "Configuration created",
				description: "Integration was added.",
				timestamp: "2026-01-01T12:00:00Z",
				severity: "success",
			},
			{
				id: "newer",
				title: "Webhook rejected",
				description: "The signature could not be verified.",
				timestamp: "2026-01-02T12:00:00Z",
				severity: "error",
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const latest = canvas.getByText("Webhook rejected");
		const earlier = canvas.getByText("Configuration created");
		await expect(latest).toBeVisible();
		await expect(
			latest.compareDocumentPosition(earlier) &
				Node.DOCUMENT_POSITION_FOLLOWING,
		).toBeTruthy();
		await expect(canvas.getByText("error")).toBeVisible();
	},
};

const pageOfActivity: IntegrationAuditEntry[] = Array.from(
	{ length: 50 },
	(_, index) => ({
		id: `entry-${index}`,
		title: "CONFIG UPDATED",
		description: `Integration configuration updated - status: ACTIVE | config_version: ${index + 2}`,
		timestamp: new Date(Date.UTC(2026, 0, 1, 12, index)).toISOString(),
		severity: "info",
	}),
);

export const OlderActivityAvailable: Story = {
	args: {
		auditEntries: pageOfActivity,
		onLoadMoreAudit: fn(),
		auditTotal: 52,
	},
	play: async ({ args, canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getAllByText("CONFIG UPDATED")).toHaveLength(50);
		const loadOlder = canvas.getByRole("button", {
			name: "Load older activity",
		});
		await expect(loadOlder).toBeEnabled();
		await expect(canvas.getByText("Showing 50 of 52")).toBeVisible();
		await userEvent.click(loadOlder);
		await expect(args.onLoadMoreAudit).toHaveBeenCalledOnce();
	},
};

export const LoadingOlderActivity: Story = {
	args: {
		auditEntries: pageOfActivity.slice(0, 3),
		onLoadMoreAudit: fn(),
		auditIsLoadingMore: true,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("button", { name: "Loading…" }),
		).toBeDisabled();
		await expect(
			canvas.queryByRole("button", { name: "Load older activity" }),
		).not.toBeInTheDocument();
	},
};

export const AllActivityLoaded: Story = {
	args: { auditEntries: pageOfActivity.slice(0, 1) },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("CONFIG UPDATED")).toBeVisible();
		await expect(
			canvas.queryByRole("button", { name: /older activity/ }),
		).not.toBeInTheDocument();
	},
};

const unbreakable = "x".repeat(180);

export const WorstCase: Story = {
	args: {
		title: `Clerk Authentication for ${"a very long product name ".repeat(4)}`,
		summary: [
			{ label: "Provider", value: "Clerk" },
			{ label: "Status", value: "ACTIVE" },
			{ label: "Activity", value: 1_234_567 },
			{ label: "Last updated", value: "January 1, 2026 12:00 PM" },
		],
		auditEntries: [
			{
				id: "unbreakable",
				title: `WEBHOOK PROCESSING FAILED ${unbreakable}`,
				description: `Webhook accepted and queued for processing - event_id: iev_${unbreakable} | external_event_id: msg_${unbreakable}`,
				timestamp: "2026-01-02T12:00:00Z",
				severity: "error",
			},
			{
				id: "empty",
				title: "",
				description: "",
				timestamp: "2026-01-01T12:00:00Z",
			},
		],
		onLoadMoreAudit: fn(),
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const loadOlder = canvas.getByRole("button", {
			name: "Load older activity",
		});
		await expect(loadOlder).toBeVisible();
		const bounds = loadOlder.getBoundingClientRect();
		await expect(bounds.right).toBeLessThanOrEqual(
			document.documentElement.clientWidth,
		);
		await expect(document.documentElement.scrollWidth).toBeLessThanOrEqual(
			document.documentElement.clientWidth,
		);
	},
};

export const OlderActivityFailedToLoad: Story = {
	args: {
		auditEntries: pageOfActivity.slice(0, 3),
		onLoadMoreAudit: fn(),
		auditTotal: 52,
		auditLoadMoreErrorMessage: "Couldn’t load older activity. Try again.",
	},
	play: async ({ args, canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getAllByText("CONFIG UPDATED")).toHaveLength(3);
		await expect(canvas.getByRole("alert")).toHaveTextContent(
			"Couldn’t load older activity. Try again.",
		);
		await userEvent.click(
			canvas.getByRole("button", { name: "Load older activity" }),
		);
		await expect(args.onLoadMoreAudit).toHaveBeenCalledOnce();
	},
};
