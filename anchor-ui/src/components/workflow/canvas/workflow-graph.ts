import type {
	WorkflowCatalogResponse,
	WorkflowStep,
	WorkflowStepStatus,
	WorkflowTriggerResponse,
} from "@/client";
import type { Edge, Node } from "@xyflow/react";
import {
	type ChainWorkflow,
	type LoopHop,
	type WorkflowDraft,
	findAction,
	findTrigger,
	operatorLabels,
	operatorNeedsValue,
	stepEmits,
	triggerVariables,
} from "../workflow-model";

export const NODE_WIDTH = 288;
const ROW_GAP = 132;
export const COLUMN_GAP = 44;
const GHOST_ROW_GAP = 80;
const GHOST_OFFSET_X = NODE_WIDTH + 96;

export type Selection =
	| { kind: "workflow" }
	| { kind: "trigger" }
	| { kind: "conditions" }
	| { kind: "step"; stepId: string };

export type RunStatusByStep = Record<string, WorkflowStepStatus>;

export interface TriggerNodeData extends Record<string, unknown> {
	title: string;
	eventType: string;
	dataFields: string[];
	selected: boolean;
	invalid: boolean;
	order: number;
}

export interface ConditionsNodeData extends Record<string, unknown> {
	count: number;
	summary: string;
	selected: boolean;
	order: number;
}

export interface StepNodeData extends Record<string, unknown> {
	step: WorkflowStep;
	index: number;
	title: string;
	group: string;
	writes: boolean;
	summary: string;
	conditionCount: number;
	selected: boolean;
	problemCount: number;
	status?: WorkflowStepStatus;
	order: number;
}

export interface AddNodeData extends Record<string, unknown> {
	index: number;
	order: number;
}

export interface WorkflowRefNodeData extends Record<string, unknown> {
	relation: "starts-this" | "started-by-this";
	workflowId: string;
	workflowName: string;
	event: string;
	order: number;
}

export type FlowEdgeTone = "default" | "active" | "done" | "chain" | "loop";

export interface FlowEdgeData extends Record<string, unknown> {
	tone: FlowEdgeTone;
	label?: string;
}

export type WorkflowGraphNode =
	| Node<TriggerNodeData, "trigger">
	| Node<ConditionsNodeData, "conditions">
	| Node<StepNodeData, "step">
	| Node<AddNodeData, "add">
	| Node<WorkflowRefNodeData, "workflowRef">;

export type WorkflowGraphEdge = Edge<FlowEdgeData, "flow">;

export const stepNodeId = (stepId: string) => `step:${stepId}`;
export const STEP_DRAG_HANDLE = "workflow-drag-handle";
const TRIGGER_ID = "trigger";
const CONDITIONS_ID = "conditions";

export function selectionNodeId(selection: Selection): string | undefined {
	if (selection.kind === "trigger") return TRIGGER_ID;
	if (selection.kind === "conditions") return CONDITIONS_ID;
	if (selection.kind === "step") return stepNodeId(selection.stepId);
	return undefined;
}
const ADD_ID = "add";

function summarize(step: WorkflowStep, catalog?: WorkflowCatalogResponse) {
	const action = findAction(catalog, step.action);
	const filled = (action?.params ?? []).filter((param) =>
		step.params[param.name]?.trim(),
	);
	const telling =
		filled.find((param) => !step.params[param.name].trim().startsWith("{{")) ??
		filled[0];
	if (!telling) return "Not configured yet";
	return `${telling.label}: ${step.params[telling.name].trim()}`;
}

export function conditionSummary(
	draft: WorkflowDraft,
	trigger: WorkflowTriggerResponse | undefined,
) {
	const conditions = draft.definition.conditions;
	if (conditions.length === 0) return "Every event starts a run";
	if (conditions.length > 1)
		return `${conditions.length} conditions must all hold`;
	const [only] = conditions;
	const field =
		triggerVariables(trigger).find((variable) => variable.path === only.field)
			?.label ?? only.field;
	const value = operatorNeedsValue(only.operator) ? ` ${only.value ?? ""}` : "";
	return `${field} ${operatorLabels[only.operator]}${value}`.trim();
}

export interface GraphInput {
	draft: WorkflowDraft;
	catalog: WorkflowCatalogResponse;
	selection: Selection;
	others: ChainWorkflow[];
	loop: LoopHop[] | null;
	problemsByStep: Record<string, number>;
	triggerInvalid: boolean;
	run?: { statuses: RunStatusByStep; revealed: number };
}

export function buildWorkflowGraph(input: GraphInput): {
	nodes: WorkflowGraphNode[];
	edges: WorkflowGraphEdge[];
} {
	const { draft, catalog, selection, others, loop, run } = input;
	const trigger = findTrigger(catalog, draft.trigger_event_type);
	const steps = draft.definition.steps;
	const nodes: WorkflowGraphNode[] = [];
	const edges: WorkflowGraphEdge[] = [];
	let order = 0;

	const startedBy = others.filter(
		(other) => other.enabled && other.emits.includes(draft.trigger_event_type),
	);
	startedBy.forEach((other, index) => {
		const id = `ref-in:${other.id}`;
		nodes.push({
			id,
			type: "workflowRef",
			position: {
				x: (index - (startedBy.length - 1) / 2) * (NODE_WIDTH + 32),
				y: -ROW_GAP,
			},
			data: {
				relation: "starts-this",
				workflowId: other.id,
				workflowName: other.name,
				event: draft.trigger_event_type,
				order: order++,
			},
			selectable: false,
			draggable: false,
		});
		edges.push({
			id: `${id}->${TRIGGER_ID}`,
			source: id,
			target: TRIGGER_ID,
			type: "flow",
			data: { tone: "chain", label: draft.trigger_event_type },
		});
	});

	nodes.push({
		id: TRIGGER_ID,
		type: "trigger",
		position: { x: 0, y: 0 },
		data: {
			title: trigger?.name ?? (draft.trigger_event_type || "Pick a trigger"),
			eventType: draft.trigger_event_type,
			dataFields: trigger?.data_fields ?? [],
			selected: selection.kind === "trigger",
			invalid: input.triggerInvalid,
			order: order++,
		},
		draggable: false,
	});
	nodes.push({
		id: CONDITIONS_ID,
		type: "conditions",
		position: { x: 0, y: ROW_GAP },
		data: {
			count: draft.definition.conditions.length,
			summary: conditionSummary(draft, trigger),
			selected: selection.kind === "conditions",
			order: order++,
		},
		draggable: false,
	});

	const revealed = run?.revealed ?? 0;
	const edgeTone = (position: number): FlowEdgeTone => {
		if (!run) return "default";
		if (position < revealed) return "done";
		if (position === revealed) return "active";
		return "default";
	};

	edges.push({
		id: `${TRIGGER_ID}->${CONDITIONS_ID}`,
		source: TRIGGER_ID,
		target: CONDITIONS_ID,
		type: "flow",
		data: { tone: run ? "done" : "default" },
	});

	let previous = CONDITIONS_ID;
	steps.forEach((step, index) => {
		const action = findAction(catalog, step.action);
		const id = stepNodeId(step.id);
		const y = ROW_GAP * (index + 2);
		const status = run && index < revealed ? run.statuses[step.id] : undefined;
		nodes.push({
			id,
			type: "step",
			position: { x: 0, y },
			data: {
				step,
				index,
				title: step.name?.trim() || action?.name || step.action,
				group: action?.group ?? "",
				writes: action?.writes ?? true,
				summary: summarize(step, catalog),
				conditionCount: step.when?.length ?? 0,
				selected: selection.kind === "step" && selection.stepId === step.id,
				problemCount: input.problemsByStep[step.id] ?? 0,
				status,
				order: order++,
			},
			draggable: true,
			dragHandle: `.${STEP_DRAG_HANDLE}`,
		});
		edges.push({
			id: `${previous}->${id}`,
			source: previous,
			target: id,
			type: "flow",
			data: { tone: edgeTone(index) },
		});
		previous = id;

		const emitted = stepEmits(catalog, step);
		let ghostRow = 0;
		for (const event of emitted) {
			for (const other of others.filter(
				(candidate) =>
					candidate.enabled && candidate.trigger_event_type === event,
			)) {
				const ghostId = `ref-out:${step.id}:${other.id}`;
				nodes.push({
					id: ghostId,
					type: "workflowRef",
					position: { x: GHOST_OFFSET_X, y: y + ghostRow * GHOST_ROW_GAP },
					data: {
						relation: "started-by-this",
						workflowId: other.id,
						workflowName: other.name,
						event,
						order: order++,
					},
					selectable: false,
					draggable: false,
				});
				edges.push({
					id: `${id}->${ghostId}`,
					source: id,
					sourceHandle: "right",
					target: ghostId,
					targetHandle: "left",
					type: "flow",
					data: { tone: "chain" },
				});
				ghostRow += 1;
			}
		}

		if (loop && emitted.includes(loop[0].emits)) {
			edges.push({
				id: `${id}->loop`,
				source: id,
				sourceHandle: "right",
				target: TRIGGER_ID,
				targetHandle: "right",
				type: "flow",
				data: { tone: "loop", label: "Loop" },
			});
		}
	});

	nodes.push({
		id: ADD_ID,
		type: "add",
		position: { x: 0, y: ROW_GAP * (steps.length + 2) },
		data: { index: steps.length, order: order++ },
		selectable: false,
		draggable: false,
	});
	edges.push({
		id: `${previous}->${ADD_ID}`,
		source: previous,
		target: ADD_ID,
		type: "flow",
		data: { tone: "default" },
	});

	return { nodes, edges };
}

const UNMEASURED_HEIGHT = ROW_GAP - COLUMN_GAP;
const heightOf = (node: WorkflowGraphNode) =>
	node.measured?.height ?? UNMEASURED_HEIGHT;

const stepsExcept = (nodes: WorkflowGraphNode[], ignoreId?: string) =>
	nodes.filter((node) => node.type === "step" && node.id !== ignoreId);

/**
 * Where a step dropped at `flowY` lands: before the first step whose vertical
 * centre is below it, else after the last. Ignoring the dragged step makes the
 * index the one `moveItem` expects.
 */
export function slotIndexAt(
	nodes: WorkflowGraphNode[],
	flowY: number,
	ignoreId?: string,
): number {
	const steps = stepsExcept(nodes, ignoreId);
	const index = steps.findIndex(
		(node) => node.position.y + heightOf(node) / 2 > flowY,
	);
	return index === -1 ? steps.length : index;
}

/** The vertical middle of the gap a step dropped at `index` would fill. */
export function slotLineY(
	nodes: WorkflowGraphNode[],
	index: number,
	ignoreId?: string,
): number {
	const steps = stepsExcept(nodes, ignoreId);
	const next = steps[index];
	if (next) return next.position.y - COLUMN_GAP / 2;
	const previous =
		steps[steps.length - 1] ?? nodes.find((node) => node.id === CONDITIONS_ID);
	return previous
		? previous.position.y + heightOf(previous) + COLUMN_GAP / 2
		: 0;
}

const withY = (node: WorkflowGraphNode, y: number): WorkflowGraphNode =>
	node.position.y === y ? node : { ...node, position: { ...node.position, y } };

/**
 * Lays the column out again on measured card heights, so a card that grew a
 * summary or chips never covers the insert button below it. A follow-up
 * workflow keeps its offset from the step that starts it.
 */
export function stackColumn(nodes: WorkflowGraphNode[]): WorkflowGraphNode[] {
	let nextY = 0;
	let anchor = { built: 0, placed: 0 };
	return nodes.map((node) => {
		if (node.type === "workflowRef") {
			if (node.data.relation === "starts-this") return node;
			return withY(node, anchor.placed + node.position.y - anchor.built);
		}
		anchor = { built: node.position.y, placed: nextY };
		nextY += heightOf(node) + COLUMN_GAP;
		return withY(node, anchor.placed);
	});
}
