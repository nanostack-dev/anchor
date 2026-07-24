import type { WebhookEventTypeDescriptor } from "@/client";
import { describe, expect, it } from "vitest";
import {
	allSubscriptions,
	coveredEventTypeCount,
	groupEventTypes,
	groupSelection,
	matchesEventTypeQuery,
	toggleEventType,
	toggleGroup,
} from "./webhook-display";

const descriptor = (
	type: string,
	group: string,
	description = "",
): WebhookEventTypeDescriptor => ({
	type,
	group,
	description,
	sample_payload: "{}",
});

const catalog: WebhookEventTypeDescriptor[] = [
	descriptor("license.created", "license", "A license was assigned."),
	descriptor("license.updated", "license", "A license changed."),
	descriptor("license.revoked", "license", "A license was revoked."),
	descriptor("plan.updated", "plan", "A plan definition changed."),
	descriptor("ping", "ping", "Synthetic delivery used to verify setup."),
];

const groups = groupEventTypes(catalog);
const licenseGroup = groups[0];
const planGroup = groups[1];
const pingGroup = groups[2];

describe("groupEventTypes", () => {
	it("offers a wildcard only where the grammar allows one", () => {
		expect(licenseGroup.wildcard).toBe("license.*");
		expect(planGroup.wildcard).toBe("plan.*");
		// `ping` is its own group and its own type, so `ping.*` would cover nothing
		// the exact type does not already cover.
		expect(pingGroup.wildcard).toBeNull();
	});
});

describe("groupSelection", () => {
	it("reads a wildcard as full coverage of its group", () => {
		expect(groupSelection(["license.*"], licenseGroup)).toBe("all");
	});

	it("reports partial coverage, which is what drives the indeterminate box", () => {
		expect(groupSelection(["license.created"], licenseGroup)).toBe("partial");
	});

	it("reports nothing for an unrelated subscription", () => {
		expect(groupSelection(["plan.*"], licenseGroup)).toBe("none");
	});
});

describe("toggleGroup", () => {
	it("collapses a whole group to its wildcard", () => {
		expect(toggleGroup([], licenseGroup)).toEqual(["license.*"]);
	});

	it("clears every entry of the group, wildcard or exact", () => {
		expect(
			toggleGroup(["license.*", "license.created", "plan.*"], licenseGroup),
		).toEqual(["plan.*"]);
	});

	it("spells out a group that has no wildcard", () => {
		expect(toggleGroup([], pingGroup)).toEqual(["ping"]);
	});
});

describe("toggleEventType", () => {
	it("collapses to the wildcard once the last type of a group is added", () => {
		const selected = ["license.created", "license.updated"];
		expect(toggleEventType(selected, licenseGroup, catalog[2]).sort()).toEqual([
			"license.*",
		]);
	});

	it("expands a covering wildcard rather than unsubscribing the whole group", () => {
		const next = toggleEventType(["license.*"], licenseGroup, catalog[1]);
		expect(next).not.toContain("license.*");
		expect(next.sort()).toEqual(["license.created", "license.revoked"]);
	});

	it("leaves other groups alone", () => {
		const next = toggleEventType(["plan.*"], licenseGroup, catalog[0]);
		expect(next).toContain("plan.*");
		expect(next).toContain("license.created");
	});
});

describe("round-tripping a saved endpoint", () => {
	it("draws a wildcard subscription as a fully checked group", () => {
		const saved = ["license.*", "ping"];
		expect(groupSelection(saved, licenseGroup)).toBe("all");
		expect(groupSelection(saved, pingGroup)).toBe("all");
		expect(groupSelection(saved, planGroup)).toBe("none");
		expect(coveredEventTypeCount(saved, catalog)).toBe(4);
	});

	it("re-emits the same entries when nothing is touched", () => {
		const saved = ["license.*"];
		const toggledOffAndOn = toggleGroup(
			toggleGroup(saved, planGroup),
			planGroup,
		);
		expect(toggledOffAndOn).toEqual(saved);
	});
});

describe("allSubscriptions", () => {
	it("covers the whole catalog with the shortest entries", () => {
		const all = allSubscriptions(groups);
		expect(all).toEqual(["license.*", "plan.*", "ping"]);
		expect(coveredEventTypeCount(all, catalog)).toBe(catalog.length);
	});
});

describe("matchesEventTypeQuery", () => {
	it("matches the type, the group and the description", () => {
		expect(matchesEventTypeQuery(catalog[2], "revok")).toBe(true);
		expect(matchesEventTypeQuery(catalog[2], "LICENSE")).toBe(true);
		expect(matchesEventTypeQuery(catalog[2], "was revoked")).toBe(true);
		expect(matchesEventTypeQuery(catalog[2], "plan")).toBe(false);
	});

	it("matches everything when the box is empty", () => {
		expect(matchesEventTypeQuery(catalog[0], "   ")).toBe(true);
	});
});
