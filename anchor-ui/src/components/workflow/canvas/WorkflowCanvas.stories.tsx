import { WorkflowOperator, WorkflowStepStatus } from "@/client";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, waitFor, within } from "storybook/test";
import {
	fixtureCatalog,
	worstCaseStep,
	worstCaseWorkflows,
} from "../workflow-fixtures";
import { emptyDraft } from "../workflow-model";
import { WorkflowCanvas } from "./WorkflowCanvas";
import { type GraphInput, buildWorkflowGraph } from "./workflow-graph";

const canvasActions = {
	actions: fixtureCatalog.actions,
	select: fn(),
	insertStep: fn(),
	openWorkflow: fn(),
};

function CanvasStory(
	input: Omit<GraphInput, "catalog" | "selection" | "triggerInvalid">,
) {
	const graph = buildWorkflowGraph({
		...input,
		catalog: fixtureCatalog,
		selection: { kind: "workflow" },
		triggerInvalid: false,
	});
	return (
		<Box className="h-[720px] w-full max-w-[880px]">
			<WorkflowCanvas
				nodes={graph.nodes}
				edges={graph.edges}
				fitKey="story"
				actions={canvasActions}
			/>
		</Box>
	);
}

const [enterprise, shortName, partnerSync] = worstCaseWorkflows;

const meta = {
	title: "Workflow/WorkflowCanvas",
	component: CanvasStory,
	parameters: { layout: "padded" },
	args: {
		draft: {
			name: "Default workspace for new organizations",
			enabled: true,
			trigger_event_type: "organization.created",
			definition: {
				conditions: [],
				steps: [
					{
						id: "workspace",
						name: "Create General workspace",
						action: "workspace.create",
						params: { name: "General" },
					},
					{ id: "tag", action: "organization.update", params: {} },
				],
			},
		},
		others: [],
		loop: null,
		problemsByStep: {},
	},
} satisfies Meta<typeof CanvasStory>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Demo: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				canvas.getByRole("button", { name: "Trigger: Organization created" }),
			).toBeVisible(),
		);
		await waitFor(() =>
			expect(
				canvas.getByRole("button", {
					name: "Step 1: Create General workspace",
				}),
			).toBeVisible(),
		);
	},
};

/**
 * Twenty-one steps, a 100-character step name with unbreakable parameter
 * values, two workflows starting this one (one called "Jo"), a long follow-up
 * name, a loop back to the trigger from every step that updates the
 * organization, a step with problems and a run paused on a failed step.
 */
export const WorstCase: Story = {
	args: {
		draft: {
			name: enterprise.name,
			enabled: true,
			trigger_event_type: "organization.created",
			definition: {
				conditions: enterprise.definition.conditions,
				steps: [worstCaseStep, ...enterprise.definition.steps],
			},
		},
		others: [
			{ ...enterprise, emits: ["organization.created"] },
			{ ...shortName, enabled: true, emits: ["organization.created"] },
			{ ...partnerSync, trigger_event_type: "organization.updated", emits: [] },
		],
		loop: [
			{
				workflowName: enterprise.name,
				trigger: "organization.created",
				emits: "organization.updated",
			},
		],
		problemsByStep: { step_2: 3 },
		run: {
			statuses: {
				[worstCaseStep.id]: WorkflowStepStatus.FAILED,
				step_1: WorkflowStepStatus.SKIPPED,
				step_2: WorkflowStepStatus.SUCCEEDED,
			},
			revealed: 2,
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				canvas.getByRole("button", { name: `Step 1: ${worstCaseStep.name}` }),
			).toBeVisible(),
		);
		await waitFor(() =>
			expect(canvas.getByRole("button", { name: "Open “Jo”" })).toBeVisible(),
		);
		await expect(
			canvas.getByRole("button", { name: /^Step 21: / }),
		).toBeInTheDocument();
		await expect(canvas.getAllByText("Loop").length).toBeGreaterThan(0);
		await expect(canvas.getByText("Failed")).toBeInTheDocument();
		await expect(canvas.queryByText("Succeeded")).toBeNull();
	},
};

export const Empty: Story = {
	args: { draft: emptyDraft() },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				canvas.getByRole("button", { name: "Trigger: Pick a trigger" }),
			).toBeVisible(),
		);
		await waitFor(() =>
			expect(canvas.getByRole("button", { name: "Add a step" })).toBeVisible(),
		);
	},
};

export const One: Story = {
	args: {
		draft: {
			name: "Jo",
			enabled: true,
			trigger_event_type: "product_user.created",
			definition: {
				conditions: [
					{
						field: "event.data.product_user_id",
						operator: WorkflowOperator.EXISTS,
					},
				],
				steps: [
					{
						id: "user",
						action: "product_user.get",
						params: {},
						when: [
							{
								field: "event.data.product_user_id",
								operator: WorkflowOperator.EXISTS,
							},
						],
					},
				],
			},
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				canvas.getByRole("button", { name: "Step 1: Read product user" }),
			).toBeVisible(),
		);
		await expect(canvas.getByText("1 condition")).toBeInTheDocument();
		await expect(canvas.getByText("Read only")).toBeInTheDocument();
	},
};
