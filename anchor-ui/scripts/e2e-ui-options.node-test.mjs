import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { parseUIOptions, uiLaunchPlan } from "./e2e-ui-options.mjs";

test("ordinary UI preserves bootstrap and exposes every project for selection", () => {
	const plan = uiLaunchPlan(parseUIOptions([]));
	assert.ok(plan.bootstrap.args.includes("--project=bootstrap"));
	assert.equal(plan.bootstrap.environment.E2E_REUSE_SERVER, "1");
	assert.equal(plan.ui.environment.E2E_REUSE_SERVER, "1");
	assert.equal(
		plan.ui.args.some((arg) => arg.startsWith("--project")),
		false,
	);
	assert.equal(plan.ui.args.includes("--headed"), false);
	assert.equal(
		plan.ui.args.some((arg) => arg.startsWith("--workers")),
		false,
	);
	assert.equal(plan.ui.environment.E2E_SLOW_MO, undefined);
	assert.ok(plan.ui.args.includes("--ui-host=127.0.0.1"));
	assert.equal(plan.uiURL, "http://127.0.0.1:9351");
});

test("visible pacing and worker options reach only the interactive child", () => {
	for (const args of [
		["--headed", "--workers", "1", "--slow-mo", "250"],
		["--slow-mo=250", "--workers=1", "--headed"],
	]) {
		const plan = uiLaunchPlan(parseUIOptions(args));
		assert.ok(plan.ui.args.includes("--headed"));
		assert.ok(plan.ui.args.includes("--workers=1"));
		assert.equal(plan.ui.environment.E2E_SLOW_MO, "250");
		assert.equal(
			plan.ui.args.some((arg) => arg.startsWith("--slow-mo")),
			false,
		);
		assert.equal(plan.bootstrap.args.includes("--headed"), false);
		assert.equal(
			plan.bootstrap.args.some((arg) => arg.startsWith("--workers")),
			false,
		);
		assert.equal(plan.bootstrap.environment.E2E_SLOW_MO, "");
	}
});

test("the app configuration receives pacing from the interactive launch plan", () => {
	const plan = uiLaunchPlan(parseUIOptions(["--slow-mo=250"]));
	for (const [child, expected] of [
		[plan.bootstrap, 0],
		[plan.ui, 250],
	]) {
		const loaded = spawnSync(
			process.execPath,
			[
				"--input-type=module",
				"--eval",
				"const { default: config } = await import(process.argv[1]); console.log(JSON.stringify(config.use.launchOptions));",
				pathToFileURL(
					path.resolve(import.meta.dirname, "../playwright.app.config.ts"),
				).href,
			],
			{
				encoding: "utf8",
				timeout: 5_000,
				env: { ...process.env, ...child.environment, PATH: "" },
			},
		);
		assert.equal(loaded.status, 0, loaded.stderr);
		assert.equal(JSON.parse(loaded.stdout).slowMo, expected);
	}
});

test("invalid and ambiguous options fail before touching a runtime", () => {
	for (const name of ["--workers", "--slow-mo"]) {
		for (const value of [
			"",
			"0",
			"-1",
			"1.5",
			"1e2",
			"NaN",
			"9007199254740992",
			"1=2",
		])
			assert.throws(
				() => parseUIOptions([`${name}=${value}`]),
				/positive safe integer/,
			);
		assert.throws(() => parseUIOptions([name]), /positive safe integer/);
		assert.throws(
			() => parseUIOptions([name, "1", name, "2"]),
			/Duplicate option/,
		);
	}
	assert.throws(() => parseUIOptions(["--unknown"]), /Unknown option/);
	assert.throws(
		() => parseUIOptions(["--headed=true"]),
		/does not accept a value/,
	);
	assert.throws(
		() => parseUIOptions(["--help", "--print-config"]),
		/Choose either/,
	);
});

function cli(args) {
	return spawnSync(
		process.execPath,
		[path.join(import.meta.dirname, "open-e2e-ui.mjs"), ...args],
		{
			encoding: "utf8",
			timeout: 5_000,
			env: {
				...process.env,
				PATH: "",
				E2E_EMAIL: "hidden-account@example.invalid",
				E2E_PASSWORD: "hidden-test-password",
			},
		},
	);
}

test("real launcher help and plan work without Docker, pnpm or Playwright startup", () => {
	const help = cli(["--help"]);
	assert.equal(help.status, 0, help.stderr);
	assert.match(help.stdout, /--headed/);
	assert.match(help.stdout, /--print-config/);
	const printed = cli([
		"--print-config",
		"--headed",
		"--workers=2",
		"--slow-mo=150",
	]);
	assert.equal(printed.status, 0, printed.stderr);
	const plan = JSON.parse(printed.stdout);
	assert.equal(plan.directory, path.resolve(import.meta.dirname, ".."));
	assert.ok(plan.ui.args.includes("--headed"));
	assert.ok(plan.ui.args.includes("--workers=2"));
	assert.equal(plan.ui.environment.E2E_SLOW_MO, "150");
	assert.equal(plan.ui.environment.E2E_EMAIL, "");
	assert.equal(plan.ui.environment.E2E_PASSWORD, "");
	assert.doesNotMatch(printed.stdout, /hidden-account|hidden-test-password/);
});

test("real launcher rejects bad arguments without starting unavailable services", () => {
	const invalid = cli(["--workers=0"]);
	assert.equal(invalid.status, 1);
	assert.match(invalid.stderr, /--workers must be a positive safe integer/);
	assert.equal(invalid.stdout, "");
	assert.doesNotMatch(invalid.stderr, /Docker|ECONNREFUSED|EADDRINUSE/);
});
