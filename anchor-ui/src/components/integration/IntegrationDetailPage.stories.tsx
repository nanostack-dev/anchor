import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";
import { IntegrationDetailPage } from "./IntegrationDetailPage";
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
