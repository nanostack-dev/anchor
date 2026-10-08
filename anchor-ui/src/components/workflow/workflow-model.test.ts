import {
	type WorkflowCatalogResponse,
	WorkflowOperator,
	WorkflowParamType,
} from "@/client";
import { describe, expect, it } from "vitest";
import { workflowRecipes } from "./recipes";
import {
	customEventType,
	describeLocation,
	draftToRequest,
	emptyDraft,
	findLoop,
	isValidCustomEvent,
	moveItem,
	newStep,
	nextStepId,
	stepEmits,
	variablesBeforeStep,
	workflowLinks,
} from "./workflow-model";

const catalog: WorkflowCatalogResponse = {
	operators: [WorkflowOperator.EQUALS],
	triggers: [
		{
			type: "organization.created",
			name: "Organization created",
			description: "",
			group_name: "Organizations",
			group_type: "internal" as never,
			data_fields: ["organization_id"],
		},
	],
	actions: [
		{
			type: "workspace.create",
			name: "Create workspace",
			description: "",
			group: "Workspaces",
			writes: true,
			emits: ["workspace.created"],
			params: [
				{
					name: "organization_id",
					label: "Organization",
					type: WorkflowParamType.ORGANIZATION,
					required: true,
					literal: false,
				},
				{
					name: "name",
					label: "Name",
					type: WorkflowParamType.TEXT,
					required: true,
					literal: false,
				},
			],
			outputs: [{ name: "workspace_id", description: "" }],
		},
	],
};

describe("workflow model", () => {
	it("prefills a parameter the trigger event carries", () => {
		const step = newStep([], catalog.actions[0], catalog.triggers[0]);
		expect(step.params).toEqual({
			organization_id: "{{event.data.organization_id}}",
			name: "",
		});
		expect(step.id).toBe("workspace");
	});

	it("never reuses a step id", () => {
		const first = { id: "workspace", action: "workspace.create", params: {} };
		expect(nextStepId([first], "workspace.create")).toBe("workspace_2");
	});

	it("offers only the outputs of earlier steps", () => {
		const draft = emptyDraft("organization.created");
		draft.definition.steps = [
			{ id: "ws", action: "workspace.create", params: {} },
			{ id: "ws_2", action: "workspace.create", params: {} },
		];
		const paths = (index: number) =>
			variablesBeforeStep(catalog, draft, index).map(
				(variable) => variable.path,
			);
		expect(paths(0)).toContain("event.data.organization_id");
		expect(paths(0)).not.toContain("steps.ws.workspace_id");
		expect(paths(1)).toContain("steps.ws.workspace_id");
		expect(paths(1)).not.toContain("steps.ws_2.workspace_id");
	});

	it("drops empty parameters and values an operator does not compare", () => {
		const draft = emptyDraft("organization.created");
		draft.name = "  Tidy  ";
		draft.definition.conditions = [
			{
				field: "event.data.organization_id",
				operator: WorkflowOperator.EXISTS,
				value: "left over",
			},
		];
		draft.definition.steps = [
			{
				id: "ws",
				action: "workspace.create",
				params: { organization_id: "x", name: " " },
				when: [],
			},
		];
		const request = draftToRequest(draft);
		expect(request.name).toBe("Tidy");
		expect(request.definition.conditions[0]).toEqual({
			field: "event.data.organization_id",
			operator: WorkflowOperator.EXISTS,
		});
		expect(request.definition.steps[0].params).toEqual({
			organization_id: "x",
		});
		expect(request.definition.steps[0].when).toBeUndefined();
	});

	it("moves a step within bounds only", () => {
		expect(moveItem(["a", "b", "c"], 0, 1)).toEqual(["b", "a", "c"]);
		expect(moveItem(["a", "b"], 0, -1)).toEqual(["a", "b"]);
	});

	it("ships recipes whose step ids are unique", () => {
		for (const recipe of workflowRecipes) {
			const ids = recipe.draft.definition.steps.map((step) => step.id);
			expect(new Set(ids).size).toBe(ids.length);
		}
	});
});

describe("loops", () => {
	const node = (
		id: string,
		trigger: string,
		emits: string[],
		enabled = true,
	) => ({ id, name: id, enabled, trigger_event_type: trigger, emits });

	it("sees a workflow that starts itself", () => {
		expect(
			findLoop(node("a", "workspace.created", ["workspace.created"]), []),
		).toHaveLength(1);
	});

	it("follows other enabled workflows and ignores disabled ones", () => {
		const candidate = node("a", "custom.ping", ["custom.pong"]);
		expect(
			findLoop(candidate, [node("b", "custom.pong", ["custom.ping"])]),
		).toHaveLength(2);
		expect(
			findLoop(candidate, [node("b", "custom.pong", ["custom.ping"], false)]),
		).toBeNull();
	});

	it("links a workflow to the ones its events start", () => {
		const links = workflowLinks([
			node("a", "organization.created", ["custom.onboarding"]),
			node("b", "custom.onboarding", []),
		]);
		expect(links.map((link) => `${link.from.id}>${link.to.id}`)).toEqual([
			"a>b",
		]);
	});

	it("names a custom event with or without its prefix", () => {
		expect(customEventType("onboarding.done")).toBe("custom.onboarding.done");
		expect(customEventType("custom.onboarding.done")).toBe(
			"custom.onboarding.done",
		);
		expect(isValidCustomEvent("custom.Bad Name")).toBe(false);
	});

	it("narrows organization.create to what its parameters write", () => {
		const step = {
			id: "org",
			action: "organization.create",
			params: {
				name: "x",
				owner_product_user_id: "{{event.data.product_user_id}}",
			},
		};
		const withEmits = {
			...catalog,
			actions: [
				{
					...catalog.actions[0],
					type: "organization.create",
					emits: [
						"organization.created",
						"organization.membership.created",
						"organization.license.updated",
					],
				},
			],
		};
		expect(stepEmits(withEmits, step)).toEqual([
			"organization.created",
			"organization.membership.created",
		]);
	});

	it("names the step a server error points at", () => {
		const draft = emptyDraft("organization.created");
		draft.definition.steps = [
			{ id: "ws", action: "workspace.create", params: {} },
		];
		expect(
			describeLocation("steps[0].params.organization_id", catalog, draft),
		).toEqual({ stepIndex: 0, label: "Step 1 · Organization" });
	});
});
