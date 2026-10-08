import { describe, expect, it } from "vitest";
import { formatFraudAmount } from "./fraud-refund-activity";

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
