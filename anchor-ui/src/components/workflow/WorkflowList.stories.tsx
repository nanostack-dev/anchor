import { WorkflowOperator } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, within } from "storybook/test";
import { WorkflowList } from "./WorkflowList";
import {
	demoRuns,
	demoWorkflows,
	fixtureCatalog,
	worstCaseRuns,
	worstCaseWorkflows,
} from "./workflow-fixtures";

const meta = {
	title: "Workflow/WorkflowList",
	component: WorkflowList,
	args: {
		workflows: demoWorkflows,
		runs: demoRuns,
		catalog: fixtureCatalog,
		onOpen: fn(),
		onCreate: fn(),
	},
} satisfies Meta<typeof WorkflowList>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Demo: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("list", { name: "Your workflows" }),
		).toBeInTheDocument();
		await expect(canvas.getAllByText("Succeeded").length).toBeGreaterThan(0);
	},
};

/**
 * Twenty steps (the maximum), a 98-character name, a trigger and an action
 * the catalog no longer lists, a running and a skipped run.
 */
export const WorstCase: Story = {
	args: { workflows: worstCaseWorkflows, runs: worstCaseRuns },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("When clerk.user.created · 2 conditions"),
		).toBeInTheDocument();
		const list = within(canvas.getByRole("list", { name: "Your workflows" }));
		await expect(list.getByText("workspace.archive")).toBeInTheDocument();
		await expect(list.getByText("+ 14 more")).toBeInTheDocument();
	},
};

export const Empty: Story = {
	args: { workflows: [], runs: [] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("No workflow has run yet."),
		).toBeInTheDocument();
		await expect(
			canvas.getByRole("button", { name: "New workflow" }),
		).toBeInTheDocument();
	},
};

export const One: Story = {
	args: {
		workflows: [
			{
				...demoWorkflows[0],
				definition: {
					...demoWorkflows[0].definition,
					conditions: [
						{ field: "event.type", operator: WorkflowOperator.EXISTS },
					],
				},
			},
		],
		runs: demoRuns,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("When organization created · 1 condition"),
		).toBeInTheDocument();
	},
};
