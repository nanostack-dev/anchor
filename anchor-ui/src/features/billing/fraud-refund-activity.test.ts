import { cleanup, render, screen, within } from "@testing-library/react";
import { createElement } from "react";
import { afterEach, describe, expect, it } from "vitest";
import { fraudDisputedFixtureState } from "./fixtures";
import {
	FraudRefundActivity,
	formatFraudAmount,
} from "./fraud-refund-activity";

afterEach(cleanup);

describe("fraud refund activity", () => {
	it("explains a disputed payment while preserving its skipped outcome", () => {
		render(
			createElement(FraudRefundActivity, { state: fraudDisputedFixtureState }),
		);
		const activity = within(
			screen.getByRole("list", { name: "Fraud refund activity" }),
		);
		expect(activity.getAllByRole("listitem")).toHaveLength(1);
		expect(activity.getByText("Skipped", { exact: true })).toBeTruthy();
		expect(
			activity.getByText("The payment is disputed and needs manual review."),
		).toBeTruthy();
		expect(activity.getByText("Amount unavailable")).toBeTruthy();
		expect(activity.queryByText("Refunded", { exact: true })).toBeNull();
		expect(activity.queryByText("charge disputed", { exact: true })).toBeNull();
	});
});

describe("fraud refund amounts", () => {
	it("keeps unknown and zero skipped amounts distinct from completed money", () => {
		expect(formatFraudAmount(0, "cad")).toBe("Amount unavailable");
		expect(formatFraudAmount(1500, "")).toBe("Amount unavailable");
	});
	it("shows minor-unit precision and separators at the policy ceiling", () => {
		expect(formatFraudAmount(1, "eur")).toBe("EUR 0.01");
		expect(formatFraudAmount(99999999, "cad")).toBe("CAD 999,999.99");
	});
	it("does not assume cents for an unsupported payment currency", () => {
		expect(formatFraudAmount(1500, "jpy")).toBe("JPY 1,500 minor units");
	});
});
