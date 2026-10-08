import {
	WorkflowFieldType,
	WorkflowOperator,
	WorkflowParamType,
} from "@/client";
import { describe, expect, it } from "vitest";
import {
	comparedValueHint,
	completeReference,
	fieldPathPattern,
	fits,
	inferFieldType,
	insertReference,
	isInsideJsonField,
	openReference,
	operatorsFor,
	parseEventData,
	serializeEventData,
	singleReference,
} from "./fields";
import type { WorkflowVariable } from "./workflow-model";

const organization: WorkflowVariable = {
	path: "event.data.organization_id",
	label: "organization_id",
	source: "Event",
	type: WorkflowFieldType.ORGANIZATION,
};

describe("fits", () => {
	it("offers an identifier parameter only fields of its kind", () => {
		expect(
			fits(WorkflowParamType.ORGANIZATION, WorkflowFieldType.ORGANIZATION),
		).toBe(true);
		expect(
			fits(WorkflowParamType.ORGANIZATION, WorkflowFieldType.PRODUCT_USER),
		).toBe(false);
	});

	it("lets a text parameter take any scalar but not an object", () => {
		expect(fits(WorkflowParamType.TEXT, WorkflowFieldType.EMAIL)).toBe(true);
		expect(fits(WorkflowParamType.TEXT, WorkflowFieldType.JSON)).toBe(false);
	});

	it("lets a JSON parameter take anything", () => {
		expect(fits(WorkflowParamType.JSON, WorkflowFieldType.JSON)).toBe(true);
	});
});

describe("references", () => {
	it("reads a value that is exactly one reference", () => {
		expect(singleReference(" {{ event.data.organization_id }} ")).toBe(
			"event.data.organization_id",
		);
		expect(singleReference("Org {{event.data.organization_id}}")).toBe(
			undefined,
		);
	});

	it("types a value from the field it references, or from its shape", () => {
		expect(
			inferFieldType("{{event.data.organization_id}}", [organization]),
		).toBe(WorkflowFieldType.ORGANIZATION);
		expect(inferFieldType("25", [])).toBe(WorkflowFieldType.NUMBER);
		expect(inferFieldType("ada@example.com", [])).toBe(WorkflowFieldType.EMAIL);
		expect(inferFieldType("pro", [])).toBe(WorkflowFieldType.TEXT);
	});

	it("finds the reference being typed at the caret", () => {
		const value = "Hello {{ev";
		expect(openReference(value, value.length)).toEqual({
			start: 6,
			query: "ev",
		});
		expect(openReference("Hello {{x}} there", 17)).toBe(undefined);
	});

	it("completes the reference being typed, eating a closing brace pair", () => {
		const value = "Hello {{ev}} there";
		expect(completeReference(value, 10, 6, "event.data.name")).toEqual({
			value: "Hello {{event.data.name}} there",
			caret: 25,
		});
	});

	it("replaces an empty value or a lone reference, and inserts into text", () => {
		expect(insertReference("", null, "a.b").value).toBe("{{a.b}}");
		expect(insertReference("{{x.y}}", 3, "a.b").value).toBe("{{a.b}}");
		expect(insertReference("Hi !", 3, "a.b")).toEqual({
			value: "Hi {{a.b}}!",
			caret: 10,
		});
	});
});

describe("event data", () => {
	it("reads typed rows and writes them back", () => {
		const rows = parseEventData(
			'{"organization_id": "{{event.data.organization_id}}", "plan": "pro"}',
			'{"organization_id": "organization"}',
		);
		expect(rows).toEqual([
			{
				key: "organization_id",
				value: "{{event.data.organization_id}}",
				type: WorkflowFieldType.ORGANIZATION,
			},
			{ key: "plan", value: "pro", type: WorkflowFieldType.TEXT },
		]);
		expect(serializeEventData(rows ?? [])).toEqual({
			data: '{"organization_id":"{{event.data.organization_id}}","plan":"pro"}',
			types: '{"organization_id":"organization","plan":"text"}',
		});
	});

	it("leaves data rows cannot show to the JSON editor", () => {
		expect(parseEventData('{"seats": 5}', "")).toBe(undefined);
		expect(parseEventData('{"meta": {{event.data.x}}}', "")).toBe(undefined);
	});

	it("drops unnamed rows and sends nothing when no row has a name", () => {
		expect(
			serializeEventData([
				{ key: " ", value: "x", type: WorkflowFieldType.TEXT },
			]),
		).toEqual({ data: "", types: "" });
	});
});

describe("typed conditions", () => {
	it("offers an identifier equality, membership and presence only", () => {
		expect(operatorsFor(WorkflowFieldType.ORGANIZATION)).toEqual([
			WorkflowOperator.EQUALS,
			WorkflowOperator.NOT_EQUALS,
			WorkflowOperator.IN,
			WorkflowOperator.EXISTS,
			WorkflowOperator.NOT_EXISTS,
		]);
	});

	it("offers a yes or no field no text comparison", () => {
		expect(operatorsFor(WorkflowFieldType.BOOLEAN)).not.toContain(
			WorkflowOperator.CONTAINS,
		);
	});

	it("offers every comparison on text and on an unknown path", () => {
		expect(operatorsFor(WorkflowFieldType.EMAIL)).toHaveLength(9);
		expect(operatorsFor(undefined)).toHaveLength(9);
	});

	it("hints the value a comparison expects", () => {
		expect(
			comparedValueHint(WorkflowFieldType.EMAIL, WorkflowOperator.ENDS_WITH),
		).toBe("@example.com");
		expect(
			comparedValueHint(
				WorkflowFieldType.ORGANIZATION,
				WorkflowOperator.EQUALS,
			),
		).toBe("Organization ID or a field");
	});

	it("accepts a path into a step's object output", () => {
		expect(fieldPathPattern.test("steps.org.metadata.plan")).toBe(true);
		expect(fieldPathPattern.test("event.data.plan")).toBe(true);
		expect(fieldPathPattern.test("plan")).toBe(false);
	});

	it("takes a typed path only inside a listed JSON field", () => {
		const listed: WorkflowVariable[] = [
			{
				path: "steps.org.metadata",
				label: "metadata",
				source: "Read organization",
				type: WorkflowFieldType.JSON,
			},
			{
				path: "steps.org.name",
				label: "name",
				source: "Read organization",
				type: WorkflowFieldType.TEXT,
			},
		];
		expect(isInsideJsonField(listed, "steps.org.metadata.plan")).toBe(true);
		expect(isInsideJsonField(listed, "steps.org.metadata")).toBe(false);
		expect(isInsideJsonField(listed, "steps.org.name.first")).toBe(false);
		expect(isInsideJsonField(listed, "steps.gone.metadata.plan")).toBe(false);
	});
});
