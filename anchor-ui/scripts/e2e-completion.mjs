export function scenarios(report) {
	const found = [];
	function visit(suite, parents = []) {
		const titles = [...parents, suite.title];
		for (const spec of suite.specs ?? [])
			for (const execution of spec.tests ?? [])
				found.push({
					identity: JSON.stringify([
						spec.file ?? suite.file,
						...titles,
						spec.title,
					]),
					execution,
				});
		for (const nested of suite.suites ?? []) visit(nested, titles);
	}
	for (const suite of report.suites ?? []) visit(suite);
	return found;
}

export function verifyCompletion(inventory, report) {
	if (
		report.errors?.length ||
		!report.stats ||
		["skipped", "flaky", "unexpected"].some((name) => report.stats[name] !== 0)
	)
		throw new Error(
			"The app run has skipped, flaky, unexpected or global failures.",
		);
	const expected = new Set(
		scenarios(inventory).map(({ identity }) => identity),
	);
	const actual = scenarios(report);
	if (!expected.size) throw new Error("The full app inventory is empty.");
	const seen = new Set();
	for (const { identity, execution } of actual) {
		if (seen.has(identity)) throw new Error(`Duplicate scenario: ${identity}`);
		seen.add(identity);
		if (
			execution.expectedStatus !== "passed" ||
			execution.results?.length !== 1 ||
			execution.results[0].status !== "passed"
		)
			throw new Error(
				`Scenario must have exactly one passed result: ${identity}`,
			);
		if (execution.results[0].retry !== 0)
			throw new Error(`Scenario used a retry: ${identity}`);
	}
	const missing = [...expected].filter((identity) => !seen.has(identity));
	const extra = [...seen].filter((identity) => !expected.has(identity));
	if (missing.length || extra.length)
		throw new Error(
			`Incomplete app run: ${missing.length} missing, ${extra.length} unexpected scenarios.`,
		);
	return { scenarios: actual.length };
}
