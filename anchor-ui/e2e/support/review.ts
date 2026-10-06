import { setTimeout as presentationDelay } from "node:timers/promises";
import type { Page, TestInfo } from "playwright/test";

async function holdReviewFrame(milliseconds: number) {
	await presentationDelay(milliseconds);
}

export async function captureReviewCheckpoint(
	page: Page,
	testInfo: TestInfo,
	name: string,
) {
	if (testInfo.project.metadata.reviewEvidence !== true) return;
	if (!/^[a-z0-9][a-z0-9-]{0,63}$/.test(name))
		throw new Error(
			"Review checkpoint names must be lowercase kebab labels of at most 64 characters",
		);
	const holdMs = testInfo.project.metadata.reviewHoldMs ?? 1_500;
	if (typeof holdMs !== "number" || !Number.isSafeInteger(holdMs) || holdMs < 0)
		throw new Error("reviewHoldMs must be a non-negative safe integer");
	await page.evaluate(async () => {
		await document.fonts.ready;
	});
	const screenshot = testInfo.outputPath(`checkpoint-${name}.png`);
	await page.screenshot({
		path: screenshot,
		animations: "disabled",
		scale: "css",
	});
	await testInfo.attach(`checkpoint-${name}`, {
		path: screenshot,
		contentType: "image/png",
	});
	await holdReviewFrame(holdMs);
}
