import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";
import { WorkflowRunHistory } from "./WorkflowRunHistory";
import { demoRuns, fixtureCatalog, worstCaseRuns } from "./workflow-fixtures";

const meta = {
	title: "Workflow/WorkflowRunHistory",
	component: WorkflowRunHistory,
	args: { runs: demoRuns, catalog: fixtureCatalog, showWorkflowName: true },
} satisfies Meta<typeof WorkflowRunHistory>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Demo: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Create workspace")).toBeInTheDocument();
	},
};

/**
 * A failed run with an unbreakable email and URL in its output, a nested
 * metadata object, a 40-character step id, an action missing from the catalog,
 * a skipped run with no event data and a run that never finished.
 */
export const WorstCase: Story = {
	args: { runs: worstCaseRuns },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getAllByText(/ORGANIZATION_MEMBERSHIP_ALREADY_EXISTS/).length,
		).toBeGreaterThan(0);
		await userEvent.click(
			canvas.getByRole("button", { name: /Read product user/ }),
		);
		await expect(
			canvas.getByText(
				"bartholomew.fitzgerald@northwind-industries-holdings.example.com",
			),
		).toBeInTheDocument();
		await userEvent.click(canvas.getByRole("button", { name: /^Jo/ }));
		await expect(
			canvas.getByText(
				"The workflow's conditions did not hold, so no step ran.",
			),
		).toBeInTheDocument();
	},
};

export const Empty: Story = {
	args: { runs: [] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText(
				"No run yet. Runs appear here as matching events happen.",
			),
		).toBeInTheDocument();
	},
};

export const Huge: Story = {
	args: {
		runs: Array.from({ length: 100 }, (_, index) => ({
			...demoRuns[0],
			id: `wfrun_${index}`,
			started_at: new Date(Date.UTC(2026, 9, 7, 9, 0, index)).toISOString(),
		})),
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("list", { name: "Runs" }).children,
		).toHaveLength(100);
	},
};
