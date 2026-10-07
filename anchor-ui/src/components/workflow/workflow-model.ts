import {
	type WorkflowActionResponse,
	type WorkflowCatalogResponse,
	type WorkflowCondition,
	WorkflowOperator,
	type WorkflowResponse,
	type WorkflowStep,
	type WorkflowTriggerResponse,
	type WorkflowWriteRequest,
} from "@/client";

export type WorkflowDraft = WorkflowWriteRequest;

export interface WorkflowVariable {
	path: string;
	label: string;
	source: string;
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
		{ path: "event.type", label: "Event type", source: "Event" },
		{ path: "workflow.id", label: "Workflow id", source: "Workflow" },
	];
	return [
		...(trigger?.data_fields ?? []).map((field) => ({
			path: `event.data.${field}`,
			label: field,
			source: "Event",
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
	const dataFields = new Set(trigger?.data_fields ?? []);
	const params = Object.fromEntries(
		action.params.map((param) => [
			param.name,
			dataFields.has(param.name) ? reference(`event.data.${param.name}`) : "",
		]),
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
