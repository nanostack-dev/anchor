import { AccessDenyReason } from "@/client";
import { client } from "@/client/client.gen";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, screen, userEvent, waitFor, within } from "storybook/test";

import {
	AccessDecision,
	CheckAccessPanel,
	type CheckResult,
} from "./CheckAccessPanel";
import { type CheckQuestion, explain } from "./access-explainer";
import {
	ACME,
	type AccessBackend,
	DAN,
	DESIGN,
	DOCUMENT_TREE,
	DOCUMENT_VIEW,
	GLOBEX,
	PERMISSIONS,
	PRICING,
	PRIYA,
	PRODUCT_ID,
	RESOURCES,
	SPECS,
	WEBSITE,
	accessRoute,
	allowed,
	denied,
	member,
	organization,
	permission,
	resource,
} from "./check-access.fixtures";
import { treeFromResponse } from "./resource-type-tree";

const TYPES = treeFromResponse(DOCUMENT_TREE);

function mockBackend(backend: AccessBackend) {
	const previous = client.getConfig();
	backend.evaluated = [];
	client.setConfig({
		baseUrl: window.location.origin,
		fetch: async (request) => {
			if (!(request instanceof Request)) throw new Error("Expected Request");
			const path = new URL(request.url).pathname;
			const body = request.method === "GET" ? {} : await request.json();
			if (path.endsWith("/resource-types/search")) {
				const types = backend.types ?? DOCUMENT_TREE;
				return Response.json({
					items: types,
					total: types.length,
					count: types.length,
				});
			}
			const reply = accessRoute(backend, request.method, path, body);
			if (reply) return reply;
			throw new Error(`Unexpected request: ${request.method} ${path}`);
		},
	});
	return () => client.setConfig(previous);
}

const BACKEND: AccessBackend = {
	organizations: [ACME, GLOBEX],
	members: [DAN, PRIYA],
	permissions: PERMISSIONS,
	resources: RESOURCES,
	evaluate: () => ({
		status: 200,
		body: allowed([{ role: "folder-viewer", type: "folder", id: DESIGN.id }]),
	}),
};

const meta = {
	title: "Product/CheckAccessPanel",
	component: CheckAccessPanel,
	args: { productId: PRODUCT_ID, types: TYPES, onHighlight: fn() },
	parameters: { layout: "padded", backend: BACKEND },
	beforeEach: ({ parameters }) => mockBackend(parameters.backend),
	decorators: [
		(Story) => (
			<StoryQuery>
				<div className="max-w-sm">
					<Story />
				</div>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof CheckAccessPanel>;

export default meta;

type Story = StoryObj<typeof meta>;
type Canvas = ReturnType<typeof within>;

async function pick(
	canvas: Canvas,
	label: string,
	option: string | RegExp,
	term?: string,
) {
	const input = canvas.getByRole("combobox", { name: label });
	await waitFor(() => expect(input).toBeEnabled());
	await userEvent.click(input);
	if (term) await userEvent.type(input, term);
	await userEvent.click(
		await screen.findByRole("option", { name: option }, { timeout: 3000 }),
	);
	await waitFor(() => expect(input).toHaveAttribute("aria-expanded", "false"));
}

async function chooseType(canvas: Canvas, name: string) {
	const trigger = canvas.getByRole("combobox", { name: "Resource type" });
	await waitFor(() => expect(trigger).toBeEnabled());
	await userEvent.click(trigger);
	await userEvent.click(await screen.findByRole("option", { name }));
	await waitFor(() => expect(trigger).toHaveTextContent(name));
}

async function chooseAll(
	canvas: Canvas,
	choice: {
		permission?: string | RegExp;
		type?: string;
		resource?: string | RegExp;
	} = {},
) {
	await pick(canvas, "Organization", "Acme Corp");
	await pick(canvas, "Member", /^Dan Okafor/, "dan");
	await pick(canvas, "Permission", choice.permission ?? /^document:view/);
	await chooseType(canvas, choice.type ?? "Document");
	if ((choice.type ?? "Document") !== "Organization")
		await pick(canvas, "Resource", choice.resource ?? /^Pricing page/);
}

async function shows(canvas: Canvas, text: string | RegExp) {
	await waitFor(() => expect(canvas.getByText(text)).toBeVisible());
}

const pathItems = (canvas: Canvas) =>
	within(
		canvas.getByRole("list", {
			name: "Path from the resource to the Organization",
		}),
	)
		.getAllByRole("listitem")
		.map((item) => item.textContent?.replace("→", "").trim());

/** Check stays disabled until all four are chosen; it asks with an explanation and names the role that grants access. */
export const AllowedThroughAFolder: Story = {
	play: async ({ args, canvasElement, parameters }) => {
		const canvas = within(canvasElement);
		const check = canvas.getByRole("button", { name: "Check" });
		await expect(check).toBeDisabled();
		await expect(
			canvas.getByRole("combobox", { name: "Member" }),
		).toBeDisabled();
		await pick(canvas, "Organization", "Acme Corp");
		await pick(canvas, "Member", /^Dan Okafor/, "dan");
		await pick(canvas, "Permission", /^document:view/);
		await expect(check).toBeDisabled();
		await chooseType(canvas, "Document");
		await expect(check).toBeDisabled();
		await pick(canvas, "Resource", /^Pricing page/, "pric");
		await expect(check).toBeEnabled();
		await userEvent.click(check);

		await shows(canvas, "Allowed");
		await expect(
			canvas.getByText("Dan Okafor has document:view on Pricing page."),
		).toBeVisible();
		await expect(
			within(canvas.getByRole("list", { name: "Granted by" })).getByRole(
				"listitem",
			),
		).toHaveTextContent("folder-viewer role on Folder Design");
		await expect(pathItems(canvas)).toEqual([
			"Pricing page (Document)",
			"Specs (Folder)",
			"Design (Folder)◀granted byfolder-viewer",
			"Website redesign (Project)",
			"Acme Corp (Organization)",
		]);
		const backend = parameters.backend as AccessBackend;
		await expect(backend.evaluated?.[0]).toEqual({
			subject: { type: "user", id: "user_dan" },
			action: { name: "document:view" },
			resource: {
				type: "document",
				id: PRICING.id,
				properties: { organization_id: ACME.id },
			},
			options: { explain: true },
		});
		await waitFor(() =>
			expect(args.onHighlight).toHaveBeenLastCalledWith({
				decision: "allowed",
				path: ["document", "folder", "project", "organization"],
				steps: [
					["folder", "document"],
					["project", "folder"],
					["organization", "project"],
				],
				granted: { folder: ["folder-viewer"] },
			}),
		);
	},
};

/** Changing any choice clears the answer and the canvas highlight, so an answer never sits next to another question. */
export const ChangingAChoiceClearsTheAnswer: Story = {
	play: async ({ args, canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas);
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		await shows(canvas, "Allowed");
		await pick(canvas, "Permission", /^document:edit/);
		await expect(canvas.queryByText("Allowed")).not.toBeInTheDocument();
		await waitFor(() =>
			expect(args.onHighlight).toHaveBeenLastCalledWith(null),
		);
	},
};

/** Asking about a project above the folder role is denied: roles reach down, never up. */
export const DeniedAboveTheRole: Story = {
	parameters: {
		backend: {
			...BACKEND,
			evaluate: () => ({ status: 200, body: denied("MISSING_PERMISSION") }),
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas, {
			permission: /^document:view/,
			type: "Project",
			resource: /^Website redesign/,
		});
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		await shows(canvas, "Denied");
		await expect(
			canvas.getByText(
				"No role Dan Okafor holds on Website redesign or above it bundles document:view.",
			),
		).toBeVisible();
		await expect(canvas.getByText("MISSING_PERMISSION")).toBeVisible();
		await expect(pathItems(canvas)).toEqual([
			"Website redesign (Project)",
			"Acme Corp (Organization)",
		]);
	},
};

/** The Organization itself is a resource: no resource search, the chain is the Organization alone. */
export const TheOrganizationItself: Story = {
	play: async ({ canvasElement, parameters }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas, { type: "Organization" });
		await expect(canvas.getByText("Checks Acme Corp itself.")).toBeVisible();
		await expect(
			canvas.queryByRole("combobox", { name: "Resource" }),
		).not.toBeInTheDocument();
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		await shows(canvas, "Allowed");
		await expect(
			(parameters.backend as AccessBackend).evaluated?.[0].resource,
		).toEqual({ type: "organization", id: ACME.id });
	},
};

/** A member without an external ID can't be named in a check, so the option says why and can't be chosen. */
export const MemberWithoutExternalId: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await pick(canvas, "Organization", "Acme Corp");
		await userEvent.click(canvas.getByRole("combobox", { name: "Member" }));
		const priya = await screen.findByRole("option", {
			name: /^priya@acme\.example/,
		});
		await expect(priya).toHaveAttribute("aria-disabled", "true");
		await expect(priya).toHaveTextContent(
			"No external ID yet, so Anchor can't name them in a check.",
		);
		await userEvent.keyboard("{Escape}");
		await waitFor(() =>
			expect(screen.queryByRole("listbox")).not.toBeInTheDocument(),
		);
	},
};

/** An Organization with no members, and a type with no registered resources, say so instead of an empty list. */
export const NothingToChoose: Story = {
	parameters: {
		backend: { ...BACKEND, members: [], resources: [] },
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await pick(canvas, "Organization", "Acme Corp");
		await expect(
			await canvas.findByText("Acme Corp has no members yet."),
		).toBeVisible();
		await chooseType(canvas, "Folder");
		await expect(
			await canvas.findByText("Acme Corp has no registered Folder resources."),
		).toBeVisible();
		await expect(canvas.getByRole("button", { name: "Check" })).toBeDisabled();
	},
};

/** A product with no organizations yet. */
export const NoOrganizations: Story = {
	parameters: { backend: { ...BACKEND, organizations: [] } },
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByText(
				"This product has no organizations yet.",
			),
		).toBeVisible();
	},
};

/** When the organizations can't load, the picker offers a retry. */
export const OrganizationsFail: Story = {
	parameters: { backend: { ...BACKEND, organizations: "fail" } },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByText("Anchor didn't return the organizations."),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Try again" }),
		).toBeVisible();
	},
};

/** While Anchor decides, Check shows it is busy. */
export const Checking: Story = {
	parameters: { backend: { ...BACKEND, evaluate: () => "pending" } },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas);
		const check = canvas.getByRole("button", { name: "Check" });
		await userEvent.click(check);
		await waitFor(() => expect(check).toHaveAttribute("aria-busy", "true"));
		await expect(canvas.getByText("Checking access…")).toBeInTheDocument();
	},
};

/** A failed call says what Anchor answered and retries the same question. */
export const CheckFails: Story = {
	parameters: {
		backend: {
			...BACKEND,
			evaluate: (_request: unknown, attempt: number) =>
				attempt === 1
					? {
							status: 500,
							body: {
								errors: [
									{ code: "UNEXPECTED_ERROR", message: "Anchor failed." },
								],
							},
						}
					: {
							status: 200,
							body: allowed([
								{ role: "folder-viewer", type: "folder", id: DESIGN.id },
							]),
						},
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas);
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		const alert = await canvas.findByRole("alert");
		await expect(alert).toHaveTextContent("Anchor didn't check access");
		await expect(alert).toHaveTextContent("Anchor failed.");
		await userEvent.click(
			within(alert).getByRole("button", { name: "Try again" }),
		);
		await shows(canvas, "Allowed");
	},
};

const UNAVAILABLE_REPLY = {
	status: 503,
	body: {
		errors: [
			{
				code: "ACCESS_DECISIONS_UNAVAILABLE",
				message: "The relationship store could not answer.",
			},
		],
	},
};

/** 503 ACCESS_DECISIONS_UNAVAILABLE: no decision was made, so the panel says so and offers a retry. */
export const DecisionsUnavailable: Story = {
	parameters: {
		backend: { ...BACKEND, evaluate: () => UNAVAILABLE_REPLY },
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas);
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		const alert = await canvas.findByRole("alert");
		await expect(alert).toHaveTextContent(
			"Access decisions are unavailable right now",
		);
		await expect(
			within(alert).getByRole("button", { name: "Try again" }),
		).toBeVisible();
		await expect(canvas.queryByText("Denied")).not.toBeInTheDocument();
	},
};

/** Try again after a 503 asks the same question again. */
export const RetriesAfterUnavailable: Story = {
	parameters: {
		backend: {
			...BACKEND,
			evaluate: (_request: unknown, attempt: number) =>
				attempt === 1
					? UNAVAILABLE_REPLY
					: { status: 200, body: denied("MISSING_PERMISSION") },
		},
	},
	play: async ({ canvasElement, parameters }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas);
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		const alert = await canvas.findByRole("alert");
		await userEvent.click(
			within(alert).getByRole("button", { name: "Try again" }),
		);
		await shows(canvas, "Denied");
		const evaluated = (parameters.backend as AccessBackend).evaluated ?? [];
		await expect(evaluated).toHaveLength(2);
		await expect(evaluated[1]).toEqual(evaluated[0]);
	},
};

let release: (() => void) | undefined;

/** An answer that arrives after a choice changed is dropped: it never shows next to the new question, and the canvas stays unlit. */
export const LateAnswerAfterAChange: Story = {
	parameters: {
		backend: {
			...BACKEND,
			evaluate: () =>
				new Promise((resolve) => {
					release = () =>
						resolve({
							status: 200,
							body: allowed([
								{ role: "folder-viewer", type: "folder", id: DESIGN.id },
							]),
						});
				}),
		},
	},
	play: async ({ args, canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas);
		const check = canvas.getByRole("button", { name: "Check" });
		await userEvent.click(check);
		await waitFor(() => expect(check).toHaveAttribute("aria-busy", "true"));
		await pick(canvas, "Permission", /^document:edit/);
		release?.();
		await waitFor(() => expect(check).not.toHaveAttribute("aria-busy"));
		await new Promise((resolve) => setTimeout(resolve, 100));
		await expect(canvas.queryByText("Allowed")).not.toBeInTheDocument();
		await expect(args.onHighlight).toHaveBeenLastCalledWith(null);
		await expect(args.onHighlight).not.toHaveBeenCalledWith(
			expect.objectContaining({ decision: "allowed" }),
		);
	},
};

const LONG = "RegulatoryComplianceEvidenceBundleWithAttachmentsAndSignatures";
const deepChain = (() => {
	const project = resource("project", "p_deep", `${LONG} project`, {
		type: "organization",
		id: ACME.id,
	});
	const folders = [project];
	for (let level = 1; level <= 12; level += 1) {
		const parent = folders[folders.length - 1];
		folders.push(
			resource("folder", `f_${level}`, `Folder level ${level} ${LONG}`, {
				type: parent.type,
				id: parent.id,
			}),
		);
	}
	const document = resource(
		"document",
		"d_deep",
		`Quarterly pricing page with an extremely long title ${LONG}`,
		{ type: "folder", id: "f_12" },
	);
	return [...folders, document];
})();

/** Worst case: long names everywhere, a fifteen-level chain and many grants. */
export const LongNamesDeepChainManyGrants: Story = {
	parameters: {
		backend: {
			...BACKEND,
			organizations: [organization("org_acme", `Acme Corporation ${LONG}`)],
			members: [
				member(
					"dan",
					`Daniel Okafor-Whitfield ${LONG}`,
					`daniel.okafor-whitfield.${LONG.toLowerCase()}@acme.example`,
				),
			],
			permissions: [permission(`document:${LONG.toLowerCase()}`, "document")],
			resources: deepChain,
			evaluate: () => ({
				status: 200,
				body: allowed([
					...Array.from({ length: 10 }, (_, index) => ({
						role: `folder-viewer-${index + 1}-${LONG.toLowerCase()}`,
						type: "folder",
						id: `f_${index + 2}`,
					})),
					{ role: "member", type: "organization", id: ACME.id },
				]),
			}),
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await pick(canvas, "Organization", /^Acme Corporation/);
		await pick(canvas, "Member", /^Daniel Okafor-Whitfield/);
		await pick(canvas, "Permission", /^document:/);
		await chooseType(canvas, "Document");
		await pick(canvas, "Resource", /^Quarterly pricing page/);
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		await shows(canvas, "Allowed");
		await expect(
			within(canvas.getByRole("list", { name: "Granted by" })).getAllByRole(
				"listitem",
			),
		).toHaveLength(11);
		await expect(pathItems(canvas)).toHaveLength(15);
		const panel = canvasElement.querySelector("section") as HTMLElement;
		await expect(panel.scrollWidth).toBeLessThanOrEqual(panel.clientWidth + 1);
	},
};

const QUESTION: CheckQuestion = {
	organization: { id: ACME.id, name: ACME.name },
	member: { label: DAN.name ?? DAN.email, externalId: "user_dan" },
	permission: { name: DOCUMENT_VIEW.name, scope: "document" },
	resource: PRICING,
};
const CHAIN = {
	links: [PRICING, SPECS, DESIGN, WEBSITE]
		.map((item) => ({ type: item.type, id: item.id, name: item.name }))
		.concat({ type: "organization", id: ACME.id, name: ACME.name }),
	complete: true,
};

function decisionFor(reason: string) {
	const result: CheckResult = {
		question: QUESTION,
		decision: denied(reason),
		chain: CHAIN,
	};
	return {
		result,
		explanation: explain(result.decision, QUESTION, CHAIN.links, TYPES),
	};
}

const REASONS = [...Object.values(AccessDenyReason), "A_REASON_ADDED_LATER"];

/** Every deny reason, in plain language with its code. */
export const EveryDenyReason: Story = {
	render: () => (
		<div className="grid gap-6">
			{REASONS.map((reason) => {
				const { result, explanation } = decisionFor(reason);
				return (
					<AccessDecision
						key={reason}
						result={result}
						explanation={explanation}
						types={TYPES}
					/>
				);
			})}
		</div>
	),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		for (const reason of REASONS) await shows(canvas, reason);
		await expect(canvas.getAllByText("Denied")).toHaveLength(REASONS.length);
		await expect(
			canvas.getByText("Dan Okafor isn't a member of Acme Corp."),
		).toBeVisible();
		await expect(
			canvas.getByText("This product declares no Document resource type."),
		).toBeVisible();
		await expect(
			canvas.getByText("Pricing page isn't registered in Acme Corp."),
		).toBeVisible();
		await expect(
			canvas.getByText("Pricing page belongs to another Organization."),
		).toBeVisible();
		await expect(
			canvas.getByText("Pricing page sits under a type it can't sit under."),
		).toBeVisible();
	},
};

/** A grant read before a change reached the decisions can be missing; the panel says so instead of showing nothing. */
export const AllowedWithoutAGrantYet: Story = {
	parameters: {
		backend: {
			...BACKEND,
			evaluate: () => ({ status: 200, body: allowed([]) }),
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas);
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		await shows(canvas, /Anchor couldn't name the role yet/);
	},
};

/** When an ancestor can't be read, the decision still shows and the path falls back to the resource. */
export const AncestorsFail: Story = {
	parameters: {
		backend: { ...BACKEND, resources: [PRICING] },
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseAll(canvas);
		await userEvent.click(canvas.getByRole("button", { name: "Check" }));
		await shows(canvas, "Allowed");
		await expect(
			canvas.getByText(/Anchor didn't return the resource's ancestors/),
		).toBeVisible();
		await expect(pathItems(canvas)).toEqual(["Pricing page (Document)"]);
	},
};
