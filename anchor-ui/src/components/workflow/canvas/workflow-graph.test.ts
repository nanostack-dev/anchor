import {
	ProductEventGroupType,
	type WorkflowCatalogResponse,
	WorkflowFieldType,
	WorkflowOperator,
	WorkflowParamType,
	WorkflowStepStatus,
} from "@/client";
import { describe, expect, it } from "vitest";
import { emptyDraft } from "../workflow-model";
import {
	buildWorkflowGraph,
	conditionSummary,
	slotIndexAt,
	slotLineY,
	stackColumn,
	stepNodeId,
} from "./workflow-graph";

const catalog: WorkflowCatalogResponse = {
	operators: [],
	triggers: [
		{
			type: "organization.created",
			name: "Organization created",
			description: "",
			group_type: ProductEventGroupType.INTERNAL,
			group_name: "Organizations",
			fields: [
				{
					name: "organization_id",
					type: WorkflowFieldType.ORGANIZATION,
					description: "",
				},
			],
			data_fields: ["organization_id"],
		},
	],
	actions: [
		{
			type: "workflow.emit",
			name: "Start other workflows",
			description: "",
			group: "Custom",
			writes: true,
			emits: [],
			params: [
				{
					name: "event",
					label: "Custom event",
					type: WorkflowParamType.CUSTOM_EVENT,
					required: true,
					literal: true,
				},
			],
			outputs: [],
		},
	],
};

function draftWithEmit(eventName: string) {
	const draft = emptyDraft("organization.created");
	draft.name = "Leader";
	draft.definition.steps = [
		{ id: "emit", action: "workflow.emit", params: { event: eventName } },
	];
	return draft;
}

const base = {
	catalog,
	selection: { kind: "workflow" } as const,
	loop: null,
	problemsByStep: {},
	triggerInvalid: false,
};

describe("workflow graph", () => {
	it("chains trigger, conditions, every step and the add node in order", () => {
		const { nodes, edges } = buildWorkflowGraph({
			...base,
			draft: draftWithEmit("onboarding.started"),
			others: [],
		});
		expect(nodes.map((node) => node.id)).toEqual([
			"trigger",
			"conditions",
			stepNodeId("emit"),
			"add",
		]);
		expect(edges.map((edge) => `${edge.source}>${edge.target}`)).toEqual([
			"trigger>conditions",
			`conditions>${stepNodeId("emit")}`,
			`${stepNodeId("emit")}>add`,
		]);
	});

	it("draws the workflows a step starts and the ones that start this one", () => {
		const { nodes } = buildWorkflowGraph({
			...base,
			draft: draftWithEmit("onboarding.started"),
			others: [
				{
					id: "wf_follow",
					name: "Follow-up",
					enabled: true,
					trigger_event_type: "custom.onboarding.started",
					emits: [],
				},
				{
					id: "wf_before",
					name: "Before",
					enabled: true,
					trigger_event_type: "workspace.created",
					emits: ["organization.created"],
				},
				{
					id: "wf_off",
					name: "Disabled",
					enabled: false,
					trigger_event_type: "custom.onboarding.started",
					emits: [],
				},
			],
		});
		expect(
			nodes
				.filter((node) => node.type === "workflowRef")
				.map((node) => node.id),
		).toEqual(["ref-in:wf_before", "ref-out:emit:wf_follow"]);
	});

	it("draws a loop back to the trigger from the step that closes it", () => {
		const { edges } = buildWorkflowGraph({
			...base,
			draft: draftWithEmit("onboarding.started"),
			others: [],
			loop: [
				{
					workflowName: "Leader",
					trigger: "organization.created",
					emits: "custom.onboarding.started",
				},
			],
		});
		const loopEdge = edges.find((edge) => edge.data?.tone === "loop");
		expect(loopEdge?.source).toBe(stepNodeId("emit"));
		expect(loopEdge?.target).toBe("trigger");
	});

	it("reveals run statuses only up to the playhead", () => {
		const draft = draftWithEmit("a");
		draft.definition.steps.push({
			id: "second",
			action: "workflow.emit",
			params: { event: "b" },
		});
		const { nodes } = buildWorkflowGraph({
			...base,
			draft,
			others: [],
			run: {
				statuses: {
					emit: WorkflowStepStatus.SIMULATED,
					second: WorkflowStepStatus.SIMULATED,
				},
				revealed: 1,
			},
		});
		const statuses = nodes
			.filter((node) => node.type === "step")
			.map((node) => node.data.status);
		expect(statuses).toEqual([WorkflowStepStatus.SIMULATED, undefined]);
	});

	it("stacks the column on measured heights and keeps follow-ups beside their step", () => {
		const { nodes } = buildWorkflowGraph({
			...base,
			draft: draftWithEmit("onboarding.started"),
			others: [
				{
					id: "wf_before",
					name: "Before",
					enabled: true,
					trigger_event_type: "workspace.created",
					emits: ["organization.created"],
				},
				{
					id: "wf_follow",
					name: "Follow-up",
					enabled: true,
					trigger_event_type: "custom.onboarding.started",
					emits: [],
				},
			],
		});
		const heights: Record<string, number> = {
			trigger: 60,
			conditions: 50,
			[stepNodeId("emit")]: 120,
			add: 40,
		};
		const stacked = stackColumn(
			nodes.map((node) => ({
				...node,
				measured: { width: 288, height: heights[node.id] ?? 56 },
			})),
		);
		expect(
			Object.fromEntries(stacked.map((node) => [node.id, node.position.y])),
		).toEqual({
			"ref-in:wf_before": -132,
			trigger: 0,
			conditions: 104,
			[stepNodeId("emit")]: 198,
			"ref-out:emit:wf_follow": 198,
			add: 362,
		});
	});

	describe("drop slots", () => {
		const draft = emptyDraft("organization.created");
		draft.definition.steps = ["first", "second", "third"].map((id) => ({
			id,
			action: "workflow.emit",
			params: { event: id },
		}));
		const column = stackColumn(
			buildWorkflowGraph({ ...base, draft, others: [] }).nodes.map((node) => ({
				...node,
				measured: { width: 288, height: 56 },
			})),
		);
		const top = (stepId: string) =>
			column.find((node) => node.id === stepNodeId(stepId))?.position.y ?? 0;

		it("lays the steps out 100 apart below the trigger and conditions", () => {
			expect([top("first"), top("second"), top("third")]).toEqual([
				200, 300, 400,
			]);
		});

		it("drops above the first step before it", () => {
			expect(slotIndexAt(column, 0)).toBe(0);
			expect(slotIndexAt(column, top("first") + 27)).toBe(0);
		});

		it("drops between two steps before the lower one", () => {
			expect(slotIndexAt(column, top("first") + 29)).toBe(1);
			expect(slotIndexAt(column, top("second") - 22)).toBe(1);
			expect(slotIndexAt(column, top("third") + 10)).toBe(2);
		});

		it("drops below the last step at the end", () => {
			expect(slotIndexAt(column, top("third") + 29)).toBe(3);
			expect(slotIndexAt(column, 5_000)).toBe(3);
		});

		it("ignores the dragged step, so the index is where it moves to", () => {
			const first = stepNodeId("first");
			expect(slotIndexAt(column, top("second") + 40, first)).toBe(1);
			expect(slotIndexAt(column, top("third") + 40, first)).toBe(2);
			expect(slotIndexAt(column, top("first") + 10, first)).toBe(0);
			expect(slotIndexAt(column, top("first") + 10, stepNodeId("third"))).toBe(
				0,
			);
		});

		it("draws the drop line in the middle of the gap the step fills", () => {
			expect(slotLineY(column, 0)).toBe(top("first") - 22);
			expect(slotLineY(column, 3)).toBe(top("third") + 56 + 22);
			expect(slotLineY(column, 2, stepNodeId("third"))).toBe(
				top("second") + 56 + 22,
			);
		});

		it("draws the drop line under the conditions when there is no step", () => {
			const empty = stackColumn(
				buildWorkflowGraph({
					...base,
					draft: emptyDraft("organization.created"),
					others: [],
				}).nodes.map((node) => ({
					...node,
					measured: { width: 288, height: 56 },
				})),
			);
			expect(slotIndexAt(empty, 1_000)).toBe(0);
			expect(slotLineY(empty, 0)).toBe(100 + 56 + 22);
		});
	});
});

describe("conditionSummary", () => {
	const withConditions = (
		conditions: { field: string; operator: WorkflowOperator; value?: string }[],
	) => {
		const draft = emptyDraft("organization.created");
		return { ...draft, definition: { ...draft.definition, conditions } };
	};
	const trigger = catalog.triggers[0];

	it("names a lone condition's field and comparison, not its path", () => {
		expect(
			conditionSummary(
				withConditions([
					{
						field: "event.data.organization_id",
						operator: WorkflowOperator.EXISTS,
						value: "stale",
					},
				]),
				trigger,
			),
		).toBe("organization_id is set");
	});

	it("keeps the path of a field the trigger does not list", () => {
		expect(
			conditionSummary(
				withConditions([
					{
						field: "steps.read.metadata.plan",
						operator: WorkflowOperator.IN,
						value: "pro, team",
					},
				]),
				trigger,
			),
		).toBe("steps.read.metadata.plan is one of pro, team");
	});

	it("counts several conditions", () => {
		expect(
			conditionSummary(
				withConditions([
					{ field: "event.type", operator: WorkflowOperator.EXISTS },
					{ field: "event.id", operator: WorkflowOperator.EXISTS },
				]),
				trigger,
			),
		).toBe("2 conditions must all hold");
	});
});
