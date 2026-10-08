import {
	type WorkflowActionResponse,
	WorkflowOperator,
	type WorkflowStep,
	WorkflowStepStatus,
} from "@/client";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useMemo, useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import {
	fixtureCatalog,
	worstCaseStep,
	worstCaseWorkflows,
} from "../workflow-fixtures";
import {
	type WorkflowDraft,
	emptyDraft,
	findTrigger,
	moveItem,
	newStep,
} from "../workflow-model";
import { WorkflowCanvas } from "./WorkflowCanvas";
import { type GraphInput, buildWorkflowGraph } from "./workflow-graph";

const canvasActions = {
	actions: fixtureCatalog.actions,
	select: fn(),
	insertStep: fn(),
	moveStep: fn(),
	openWorkflow: fn(),
};

function EditableCanvasStory({ draft: initial }: { draft: WorkflowDraft }) {
	const [draft, setDraft] = useState(initial);
	const graph = useMemo(
		() =>
			buildWorkflowGraph({
				draft,
				catalog: fixtureCatalog,
				selection: { kind: "workflow" },
				others: [],
				loop: null,
				problemsByStep: {},
				triggerInvalid: false,
			}),
		[draft],
	);
	const actions = useMemo(() => {
		const setSteps = (change: (steps: WorkflowStep[]) => WorkflowStep[]) =>
			setDraft((current) => ({
				...current,
				definition: {
					...current.definition,
					steps: change(current.definition.steps),
				},
			}));
		return {
			actions: fixtureCatalog.actions,
			select: () => {},
			insertStep: (index: number, action: WorkflowActionResponse) =>
				setSteps((steps) => [
					...steps.slice(0, index),
					newStep(
						steps,
						action,
						findTrigger(fixtureCatalog, initial.trigger_event_type),
					),
					...steps.slice(index),
				]),
			moveStep: (from: number, to: number) =>
				setSteps((steps) => moveItem(steps, from, to)),
		};
	}, [initial.trigger_event_type]);
	return (
		<Box className="h-[720px] w-full max-w-[880px]">
			<WorkflowCanvas
				nodes={graph.nodes}
				edges={graph.edges}
				fitKey={String(draft.definition.steps.length)}
				actions={actions}
			/>
		</Box>
	);
}

const center = (element: Element) => {
	const box = element.getBoundingClientRect();
	return { x: box.left + box.width / 2, y: box.top + box.height / 2 };
};

const nextFrame = () =>
	new Promise((resolve) => requestAnimationFrame(() => resolve(undefined)));

async function openPalette(canvas: ReturnType<typeof within>) {
	const toggle = canvas.getByRole("button", { name: "Steps" });
	if (toggle.getAttribute("aria-expanded") !== "true")
		await userEvent.click(toggle);
	await waitFor(() =>
		expect(canvas.getByRole("region", { name: "Steps to add" })).toBeVisible(),
	);
}

/** Native drag and drop, the events a browser fires for a palette drag. */
async function dropFromPalette(
	canvas: ReturnType<typeof within>,
	actionName: string,
	target: HTMLElement,
) {
	const item = canvas.getByRole("button", { name: `Add step: ${actionName}` });
	const dataTransfer = new DataTransfer();
	const fire = (type: string, element: Element) =>
		element.dispatchEvent(
			new DragEvent(type, {
				bubbles: true,
				cancelable: true,
				dataTransfer,
				clientX: center(target).x,
				clientY: center(target).y,
			}),
		);
	fire("dragstart", item);
	await nextFrame();
	fire("dragenter", target);
	fire("dragover", target);
	await nextFrame();
	fire("drop", target);
	fire("dragend", item);
}

/** React Flow drags a node on mouse events, through the node's drag handle. */
async function dragHandleTo(handle: HTMLElement, to: { x: number; y: number }) {
	const from = center(handle);
	const fire = (type: string, target: EventTarget, x: number, y: number) =>
		target.dispatchEvent(
			new MouseEvent(type, {
				bubbles: true,
				cancelable: true,
				view: window,
				button: 0,
				buttons: type === "mouseup" ? 0 : 1,
				clientX: x,
				clientY: y,
			}),
		);
	fire("mousedown", handle, from.x, from.y);
	fire("mousemove", document, from.x, from.y + 2);
	await nextFrame();
	const moves = 8;
	for (let move = 1; move <= moves; move++) {
		fire(
			"mousemove",
			document,
			from.x + ((to.x - from.x) * move) / moves,
			from.y + ((to.y - from.y) * move) / moves,
		);
		await nextFrame();
	}
	fire("mouseup", document, to.x, to.y);
	await nextFrame();
}

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

/** A canvas that keeps its edits, to try dragging steps in and around. */
export const Editable: Story = {
	render: (args) => <EditableCanvasStory draft={args.draft} />,
};

/**
 * Steps dropped from the palette on an insert slot, on the pane and on the
 * add node, one appended by click, then reordered with the keyboard and by
 * dragging a step's handle.
 */
export const DragAndDrop: Story = {
	render: (args) => <EditableCanvasStory draft={args.draft} />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const step = (position: number, name: string) =>
			canvas.getByRole("button", { name: `Step ${position}: ${name}` });
		await waitFor(() =>
			expect(step(1, "Create General workspace")).toBeVisible(),
		);

		await openPalette(canvas);
		await dropFromPalette(
			canvas,
			"Read product user",
			canvas.getByRole("button", { name: "Insert a step before step 1" }),
		);
		await waitFor(() => expect(step(1, "Read product user")).toBeVisible());
		await expect(step(2, "Create General workspace")).toBeInTheDocument();

		await openPalette(canvas);
		await dropFromPalette(
			canvas,
			"Add member",
			canvas.getByRole("button", { name: "Trigger: Organization created" }),
		);
		await waitFor(() => expect(step(1, "Add member")).toBeVisible());

		await openPalette(canvas);
		await userEvent.click(
			canvas.getByRole("button", { name: "Add step: Send email" }),
		);
		await waitFor(() => expect(step(5, "Send email")).toBeInTheDocument());

		await openPalette(canvas);
		await dropFromPalette(
			canvas,
			"Create workspace",
			canvas.getByRole("button", { name: "Add a step" }),
		);
		await waitFor(() =>
			expect(step(6, "Create workspace")).toBeInTheDocument(),
		);

		canvas.getByRole("button", { name: "Reorder step 1" }).focus();
		await userEvent.keyboard("{ArrowDown}");
		await waitFor(() => expect(step(2, "Add member")).toBeInTheDocument());
		await expect(step(1, "Read product user")).toBeInTheDocument();
		await waitFor(() =>
			expect(
				canvas.getByRole("button", { name: "Reorder step 2" }),
			).toHaveFocus(),
		);
		await expect(
			canvas.getByText("Step moved to position 2", { exact: true }),
		).toBeInTheDocument();
		await userEvent.keyboard("{ArrowUp}");
		await waitFor(() => expect(step(1, "Add member")).toBeInTheDocument());

		const below = step(2, "Read product user").getBoundingClientRect();
		await dragHandleTo(canvas.getByRole("button", { name: "Reorder step 1" }), {
			x: below.left + below.width / 2,
			y: below.top + below.height * 0.8,
		});
		await waitFor(() => expect(step(1, "Read product user")).toBeVisible());
		await expect(step(2, "Add member")).toBeVisible();
		await expect(step(3, "Create General workspace")).toBeVisible();
	},
};
