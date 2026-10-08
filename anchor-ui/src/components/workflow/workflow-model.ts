import {
	type WorkflowActionResponse,
	type WorkflowCatalogResponse,
	type WorkflowCondition,
	WorkflowFieldType,
	WorkflowOperator,
	type WorkflowResponse,
	type WorkflowStep,
	type WorkflowTriggerResponse,
	type WorkflowWriteRequest,
} from "@/client";

import { expectedTypeLabel, fits } from "./fields";

export type WorkflowDraft = WorkflowWriteRequest;

export interface WorkflowVariable {
	path: string;
	label: string;
	source: string;
	type: WorkflowFieldType;
	description?: string;
}

export const operatorLabels: Record<WorkflowOperator, string> = {
	[WorkflowOperator.EQUALS]: "equals",
	[WorkflowOperator.NOT_EQUALS]: "does not equal",
	[WorkflowOperator.CONTAINS]: "contains",
	[WorkflowOperator.NOT_CONTAINS]: "does not contain",
	[WorkflowOperator.STARTS_WITH]: "starts with",
	[WorkflowOperator.ENDS_WITH]: "ends with",
	[WorkflowOperator.IN]: "is one of",
	[WorkflowOperator.EXISTS]: "is set",
	[WorkflowOperator.NOT_EXISTS]: "is empty",
};

export function operatorNeedsValue(operator: WorkflowOperator): boolean {
	return (
		operator !== WorkflowOperator.EXISTS &&
		operator !== WorkflowOperator.NOT_EXISTS
	);
}

export function emptyDraft(trigger = ""): WorkflowDraft {
	return {
		name: "",
		description: "",
		enabled: true,
		trigger_event_type: trigger,
		definition: { conditions: [], steps: [] },
	};
}

export function draftFromResponse(workflow: WorkflowResponse): WorkflowDraft {
	return {
		name: workflow.name,
		description: workflow.description ?? "",
		enabled: workflow.enabled,
		trigger_event_type: workflow.trigger_event_type,
		definition: {
			conditions: workflow.definition.conditions ?? [],
			steps: workflow.definition.steps.map((step) => ({
				...step,
				when: step.when ?? [],
			})),
		},
	};
}

function cleanCondition(condition: WorkflowCondition): WorkflowCondition {
	return operatorNeedsValue(condition.operator)
		? { ...condition, value: condition.value ?? "" }
		: { field: condition.field, operator: condition.operator };
}

function cleanParams(params: Record<string, string>): Record<string, string> {
	return Object.fromEntries(
		Object.entries(params).filter(([, value]) => value.trim() !== ""),
	);
}

export function draftToRequest(draft: WorkflowDraft): WorkflowWriteRequest {
	return {
		name: draft.name.trim(),
		description: draft.description?.trim() || undefined,
		enabled: draft.enabled,
		trigger_event_type: draft.trigger_event_type,
		definition: {
			conditions: draft.definition.conditions.map(cleanCondition),
			steps: draft.definition.steps.map((step) => ({
				id: step.id,
				name: step.name?.trim() || undefined,
				action: step.action,
				params: cleanParams(step.params),
				when: step.when?.length ? step.when.map(cleanCondition) : undefined,
				continue_on_error: step.continue_on_error || undefined,
			})),
		},
	};
}

export function findTrigger(
	catalog: WorkflowCatalogResponse | undefined,
	type: string,
): WorkflowTriggerResponse | undefined {
	return catalog?.triggers.find((trigger) => trigger.type === type);
}

export function findAction(
	catalog: WorkflowCatalogResponse | undefined,
	type: string,
): WorkflowActionResponse | undefined {
	return catalog?.actions.find((action) => action.type === type);
}

export function triggerVariables(
	trigger: WorkflowTriggerResponse | undefined,
): WorkflowVariable[] {
	const base: WorkflowVariable[] = [
		{
			path: "event.type",
			label: "Event type",
			source: "Event",
			type: WorkflowFieldType.TEXT,
		},
		{
			path: "event.id",
			label: "Event id",
			source: "Event",
			type: WorkflowFieldType.TEXT,
		},
		{
			path: "workflow.id",
			label: "Workflow id",
			source: "Workflow",
			type: WorkflowFieldType.TEXT,
		},
		{
			path: "workflow.name",
			label: "Workflow name",
			source: "Workflow",
			type: WorkflowFieldType.TEXT,
		},
	];
	return [
		...(trigger?.fields ?? []).map((field) => ({
			path: `event.data.${field.name}`,
			label: field.name,
			source: "Event",
			type: field.type,
			description: field.description,
		})),
		...base,
	];
}

export function variablesBeforeStep(
	catalog: WorkflowCatalogResponse | undefined,
	draft: WorkflowDraft,
	stepIndex: number,
): WorkflowVariable[] {
	const trigger = findTrigger(catalog, draft.trigger_event_type);
	const fromSteps = draft.definition.steps
		.slice(0, stepIndex)
		.flatMap((step) => {
			const action = findAction(catalog, step.action);
			return (action?.outputs ?? []).map((output) => ({
				path: `steps.${step.id}.${output.name}`,
				label: output.name,
				source: step.name || action?.name || step.id,
				type: output.type,
				description: output.description,
			}));
		});
	return [...triggerVariables(trigger), ...fromSteps];
}

export function reference(path: string): string {
	return `{{${path}}}`;
}

export function nextStepId(steps: WorkflowStep[], action: string): string {
	const base =
		action
			.split(".")
			.shift()
			?.replace(/[^a-z0-9_]/g, "_") || "step";
	const taken = new Set(steps.map((step) => step.id));
	if (!taken.has(base)) return base;
	let suffix = 2;
	while (taken.has(`${base}_${suffix}`)) suffix += 1;
	return `${base}_${suffix}`;
}

export function newStep(
	steps: WorkflowStep[],
	action: WorkflowActionResponse,
	trigger: WorkflowTriggerResponse | undefined,
): WorkflowStep {
	const fields = trigger?.fields ?? [];
	const params = Object.fromEntries(
		action.params.map((param) => {
			const sameName = fields.find((field) => field.name === param.name);
			const fitting = expectedTypeLabel(param.type)
				? fields.filter((field) => fits(param.type, field.type))
				: [];
			const pick = sameName ?? (fitting.length === 1 ? fitting[0] : undefined);
			return [
				param.name,
				pick && !param.literal ? reference(`event.data.${pick.name}`) : "",
			];
		}),
	);
	return {
		id: nextStepId(steps, action.type),
		action: action.type,
		params,
		when: [],
	};
}

export function moveItem<T>(items: T[], from: number, to: number): T[] {
	if (to < 0 || to >= items.length) return items;
	const next = [...items];
	const [moved] = next.splice(from, 1);
	next.splice(to, 0, moved);
	return next;
}

export function sampleEventData(
	trigger: WorkflowTriggerResponse | undefined,
	previous: Record<string, string> = {},
): Record<string, string> {
	return Object.fromEntries(
		(trigger?.data_fields ?? []).map((field) => [field, previous[field] ?? ""]),
	);
}

export function groupBy<T>(
	items: T[],
	key: (item: T) => string,
): Array<[string, T[]]> {
	const groups = new Map<string, T[]>();
	for (const item of items) {
		const name = key(item);
		groups.set(name, [...(groups.get(name) ?? []), item]);
	}
	return [...groups.entries()];
}

export const CUSTOM_EVENT_PREFIX = "custom.";
const customEventPattern = /^custom\.[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$/;

export function customEventType(name: string): string {
	const trimmed = name.trim();
	return trimmed.startsWith(CUSTOM_EVENT_PREFIX)
		? trimmed
		: `${CUSTOM_EVENT_PREFIX}${trimmed}`;
}

export function isValidCustomEvent(eventType: string): boolean {
	return eventType.length <= 100 && customEventPattern.test(eventType);
}

export function stepEmits(
	catalog: WorkflowCatalogResponse | undefined,
	step: WorkflowStep,
): string[] {
	if (step.action === "workflow.emit") {
		const name = step.params.event?.trim();
		return name ? [customEventType(name)] : [];
	}
	const emits = findAction(catalog, step.action)?.emits ?? [];
	if (step.action !== "organization.create") return emits;
	return emits.filter(
		(eventType) =>
			eventType === "organization.created" ||
			(eventType === "organization.membership.created" &&
				Boolean(step.params.owner_product_user_id?.trim())) ||
			(eventType === "organization.license.updated" &&
				Boolean(step.params.license_template_id?.trim())),
	);
}

export function draftEmits(
	catalog: WorkflowCatalogResponse | undefined,
	draft: WorkflowDraft,
): string[] {
	return [
		...new Set(
			draft.definition.steps.flatMap((step) => stepEmits(catalog, step)),
		),
	];
}

export interface ChainWorkflow {
	id: string;
	name: string;
	enabled: boolean;
	trigger_event_type: string;
	emits: string[];
}

export interface LoopHop {
	workflowName: string;
	trigger: string;
	emits: string;
}

export function findLoop(
	candidate: ChainWorkflow,
	others: ChainWorkflow[],
): LoopHop[] | null {
	if (!candidate.enabled) return null;
	const enabled = others.filter(
		(other) => other.enabled && other.id !== candidate.id,
	);
	const hop = (workflow: ChainWorkflow, emits: string): LoopHop => ({
		workflowName: workflow.name || "This workflow",
		trigger: workflow.trigger_event_type,
		emits,
	});
	const queue = candidate.emits.map((emits) => ({
		event: emits,
		path: [hop(candidate, emits)],
	}));
	const visited = new Set<string>();
	while (queue.length > 0) {
		const current = queue.shift();
		if (!current) break;
		if (current.event === candidate.trigger_event_type) return current.path;
		if (visited.has(current.event)) continue;
		visited.add(current.event);
		for (const next of enabled.filter(
			(other) => other.trigger_event_type === current.event,
		)) {
			for (const emits of next.emits) {
				queue.push({ event: emits, path: [...current.path, hop(next, emits)] });
			}
		}
	}
	return null;
}

export function describeLoop(path: LoopHop[]): string {
	return `${path
		.map(
			(hop) => `“${hop.workflowName}” (on ${hop.trigger}) emits ${hop.emits}`,
		)
		.join(" → ")} → back to the start`;
}

export interface WorkflowLink {
	from: ChainWorkflow;
	event: string;
	to: ChainWorkflow;
}

export function workflowLinks(workflows: ChainWorkflow[]): WorkflowLink[] {
	return workflows.flatMap((from) =>
		from.emits.flatMap((event) =>
			workflows
				.filter((to) => to.trigger_event_type === event && to.enabled)
				.map((to) => ({ from, event, to })),
		),
	);
}

export function describeLocation(
	location: string,
	catalog: WorkflowCatalogResponse | undefined,
	draft: WorkflowDraft,
): { stepIndex?: number; label: string } {
	const stepMatch = location.match(/^steps\[(\d+)\](?:\.(\w+)(?:\.(\w+))?)?/);
	if (stepMatch) {
		const stepIndex = Number(stepMatch[1]);
		const step = draft.definition.steps[stepIndex];
		const action = findAction(catalog, step?.action ?? "");
		const param = action?.params.find((item) => item.name === stepMatch[3]);
		const part =
			stepMatch[2] === "params"
				? (param?.label ?? stepMatch[3])
				: stepMatch[2] === "when"
					? "its conditions"
					: undefined;
		return {
			stepIndex,
			label: `Step ${stepIndex + 1}${part ? ` · ${part}` : ""}`,
		};
	}
	if (location.startsWith("conditions")) return { label: "Conditions" };
	if (location.startsWith("trigger_event_type")) return { label: "Trigger" };
	return { label: location };
}
