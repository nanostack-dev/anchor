import type { ProductResourceTypeResponse } from "@/client";
import { client } from "@/client/client.gen";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, waitFor, within } from "storybook/test";

import { ResourceTypesEditor } from "./ResourceTypesEditor";

const PRODUCT_ID = "prd_2Nq8xKf3pLmR";

function resourceType(
	slug: string,
	name: string,
	parentTypes: string[] = ["organization"],
	nests = false,
): ProductResourceTypeResponse {
	return {
		id: `rtype_${slug}`,
		product_id: PRODUCT_ID,
		slug,
		name,
		parent_types: parentTypes,
		nests_in_itself: nests,
		custom_roles: false,
		built_in: slug === "organization",
		created_at: "2026-10-24T09:12:31Z",
		updated_at: "2026-10-24T09:12:31Z",
	};
}

const ORGANIZATION = resourceType("organization", "Organization", []);
const DOCUMENT_TREE = [
	ORGANIZATION,
	resourceType("project", "Project"),
	resourceType("folder", "Folder", ["project"], true),
	resourceType("document", "Document", ["folder", "project"]),
];
const LONG_NAME =
	"RegulatoryComplianceEvidenceBundleWithAttachmentsAndSignaturesForAuditors";

interface Backend {
	types: ProductResourceTypeResponse[] | "pending" | "fail";
	usage?: Record<string, { roles: number; permissions: number }>;
	publish?: { status: number; body: unknown };
	switchReply?: { status: number; body: unknown };
	roles?: { id: string; name: string }[];
}

const published: unknown[] = [];

function mockBackend(backend: Backend) {
	published.length = 0;
	let tree = backend.types;
	const previous = client.getConfig();
	client.setConfig({
		baseUrl: window.location.origin,
		fetch: async (request) => {
			if (!(request instanceof Request)) throw new Error("Expected Request");
			const path = new URL(request.url).pathname;
			const body = request.method === "GET" ? {} : await request.json();
			if (path.endsWith("/resource-types/search")) {
				if (tree === "pending") return new Promise<Response>(() => {});
				if (tree === "fail")
					return Response.json(
						{ errors: [{ code: "UNEXPECTED_ERROR", message: "Unavailable" }] },
						{ status: 500 },
					);
				return Response.json({
					items: tree,
					total: tree.length,
					count: tree.length,
				});
			}
			const patched = path.match(/\/resource-types\/([^/]+)$/);
			if (patched && request.method === "PATCH") {
				published.push({ patch: patched[1], ...body });
				if (backend.switchReply)
					return Response.json(backend.switchReply.body, {
						status: backend.switchReply.status,
					});
				if (Array.isArray(tree))
					tree = tree.map((type) =>
						type.id === patched[1] ? { ...type, ...body } : type,
					);
				return Response.json(
					Array.isArray(tree)
						? tree.find((type) => type.id === patched[1])
						: {},
				);
			}
			if (path.endsWith("/resource-types") && request.method === "PUT") {
				published.push(body);
				if (backend.publish)
					return Response.json(backend.publish.body, {
						status: backend.publish.status,
					});
				const items = body.resource_types as {
					slug: string;
					name: string;
					parent_types: string[];
					nests_in_itself?: boolean;
				}[];
				tree = [
					ORGANIZATION,
					...items.map((item) =>
						resourceType(
							item.slug,
							item.name,
							item.parent_types,
							item.nests_in_itself,
						),
					),
				];
				return Response.json({ resource_types: tree });
			}
			const slug = body?.filter?.resource_types?.[0] as string | undefined;
			if (path.endsWith("/roles/search")) {
				if (body?.filter?.ids)
					return Response.json({
						items: (backend.roles ?? []).map((role) => ({
							...role,
							product_id: PRODUCT_ID,
							resource_type: "project",
							permissions: [],
							created_at: ORGANIZATION.created_at,
							updated_at: ORGANIZATION.updated_at,
						})),
						total: backend.roles?.length ?? 0,
						count: backend.roles?.length ?? 0,
					});
				const total = slug ? (backend.usage?.[slug]?.roles ?? 0) : 0;
				return Response.json({ items: [], total, count: 0 });
			}
			if (path.endsWith("/resource-permissions/search")) {
				const total = slug ? (backend.usage?.[slug]?.permissions ?? 0) : 0;
				return Response.json({ items: [], total, count: 0 });
			}
			throw new Error(`Unexpected request: ${request.method} ${path}`);
		},
	});
	return () => client.setConfig(previous);
}

const meta = {
	title: "Product/ResourceTypesEditor",
	component: ResourceTypesEditor,
	args: { productId: PRODUCT_ID },
	parameters: { layout: "padded", backend: { types: DOCUMENT_TREE } },
	beforeEach: ({ parameters }) => mockBackend(parameters.backend as Backend),
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
} satisfies Meta<typeof ResourceTypesEditor>;

export default meta;

type Story = StoryObj<typeof meta>;

const typeNode = (canvasElement: HTMLElement, name: string) =>
	within(canvasElement).findByRole("button", { name });

async function startEditing(canvasElement: HTMLElement) {
	await userEvent.click(
		await within(canvasElement).findByRole("button", {
			name: "Edit resource types",
		}),
	);
}

const inspector = (canvasElement: HTMLElement) =>
	within(
		within(canvasElement).getByRole("complementary", {
			name: "Resource type details",
		}),
	);

/** Reading is the default: the tree with no editing controls. */
export const ReadTree: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await typeNode(canvasElement, "Document");
		await expect(
			await typeNode(canvasElement, "Organization"),
		).toHaveAccessibleDescription("Built in. Slug organization.");
		await expect(
			await typeNode(canvasElement, "Folder"),
		).toHaveAccessibleDescription("Slug folder. Nests in itself.");
		await expect(canvas.getByText("nests in itself")).toBeVisible();
		await expect(
			canvas.queryAllByRole("button", { name: /^Add a type under/ }),
		).toHaveLength(0);
		await expect(
			canvasElement.querySelectorAll(".react-flow__edge"),
		).toHaveLength(4);
		await userEvent.click(await typeNode(canvasElement, "Document"));
		await expect(
			inspector(canvasElement).getByText("Folder, Project"),
		).toBeVisible();
		await expect(
			inspector(canvasElement).getByText("0 roles · 0 permissions"),
		).toBeVisible();
	},
};

/** A product that declared nothing yet shows only the locked root. */
export const OneType: Story = {
	parameters: { backend: { types: [ORGANIZATION] } },
	play: async ({ canvasElement }) => {
		await userEvent.click(await typeNode(canvasElement, "Organization"));
		await expect(
			inspector(canvasElement).getByText(/Built in. Organization is the root/),
		).toBeVisible();
		await startEditing(canvasElement);
		await expect(
			inspector(canvasElement).queryByRole("button", { name: /^Delete/ }),
		).toBeNull();
		await expect(
			within(canvasElement).getByRole("button", { name: "Publish" }),
		).toBeDisabled();
	},
};

/** The largest tree a product may declare: fifty types and the root. */
export const FiftyOneTypes: Story = {
	parameters: {
		backend: {
			types: [
				ORGANIZATION,
				...Array.from({ length: 50 }, (_, index) =>
					resourceType(`type_${index}`, `Resource type number ${index}`),
				),
			],
		},
	},
	play: async ({ canvasElement }) => {
		await typeNode(canvasElement, "Resource type number 49");
		await expect(
			within(canvasElement).getAllByRole("button", {
				name: /^(Organization|Resource type number \d+)$/,
			}),
		).toHaveLength(51);
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Organization"));
		await expect(
			inspector(canvasElement).getByRole("button", {
				name: "Add child type",
			}),
		).toBeDisabled();
		await expect(
			inspector(canvasElement).getByText(
				"A product declares at most 50 resource types besides Organization.",
			),
		).toBeVisible();
	},
};

/** Long unbroken names stay inside their node and wrap in the inspector. */
export const LongNames: Story = {
	parameters: {
		backend: {
			types: [
				ORGANIZATION,
				resourceType("evidence_bundle", LONG_NAME),
				resourceType(
					"x".repeat(49),
					`${LONG_NAME}Again`,
					["evidence_bundle"],
					true,
				),
			],
		},
	},
	play: async ({ canvasElement }) => {
		const node = await typeNode(canvasElement, LONG_NAME);
		await expect(node.scrollWidth).toBeLessThanOrEqual(node.clientWidth + 1);
		await userEvent.click(node);
		const details = within(canvasElement).getByRole("complementary", {
			name: "Resource type details",
		});
		await expect(details.scrollWidth).toBeLessThanOrEqual(
			details.clientWidth + 1,
		);
	},
};

/** The deepest tree Anchor accepts: five levels counting Organization. */
export const DeepTree: Story = {
	parameters: {
		backend: {
			types: [
				ORGANIZATION,
				resourceType("workspace", "Workspace"),
				resourceType("project", "Project", ["workspace"]),
				resourceType("folder", "Folder", ["project"], true),
				resourceType("document", "Document", ["folder"]),
			],
		},
	},
	play: async ({ canvasElement }) => {
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Document"));
		await expect(
			inspector(canvasElement).getByText(
				"The tree is at most 5 levels deep, counting Organization.",
			),
		).toBeVisible();
		await expect(
			within(canvasElement).getByRole("button", {
				name: "Add a type under Document",
			}),
		).toHaveAttribute("aria-disabled", "true");
	},
};

/** A type under ten parents draws one edge from each. */
export const ManyParents: Story = {
	parameters: {
		backend: {
			types: [
				ORGANIZATION,
				...Array.from({ length: 10 }, (_, index) =>
					resourceType(`area_${index}`, `Area ${index}`),
				),
				resourceType(
					"note",
					"Note",
					Array.from({ length: 10 }, (_, index) => `area_${index}`),
				),
			],
		},
	},
	play: async ({ canvasElement }) => {
		await userEvent.click(await typeNode(canvasElement, "Note"));
		await expect(
			canvasElement.querySelectorAll(
				'.react-flow__edge[data-id$="->rtype_note"]',
			),
		).toHaveLength(10);
		await expect(
			inspector(canvasElement).getByText(
				Array.from({ length: 10 }, (_, index) => `Area ${index}`).join(", "),
			),
		).toBeVisible();
	},
};

export const Loading: Story = {
	parameters: { backend: { types: "pending" } },
	play: async ({ canvasElement }) => {
		await expect(
			within(canvasElement).getByRole("status", {
				name: "Loading resource types",
			}),
		).toBeVisible();
	},
};

export const LoadError: Story = {
	parameters: { backend: { types: "fail" } },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByText("Couldn't load the resource types"),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Try again" }),
		).toBeEnabled();
	},
};

/** Anchor refuses a tree that leaves roles out of scope and names them. */
export const PublishError: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			roles: [{ id: "role_1", name: "Project editor" }],
			publish: {
				status: 409,
				body: {
					errors: [
						{
							code: "RESOURCE_TYPE_IN_USE",
							message: "Roles would bundle permissions outside their scope.",
							metadata: { blocked_by: { roles: 1 }, role_ids: ["role_1"] },
						},
					],
				},
			},
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Document"));
		await userEvent.click(
			inspector(canvasElement).getByRole("checkbox", { name: "Organization" }),
		);
		await userEvent.click(
			inspector(canvasElement).getByRole("checkbox", { name: "Folder" }),
		);
		await userEvent.click(
			inspector(canvasElement).getByRole("checkbox", { name: "Project" }),
		);
		await expect(canvas.getByText("Draft: 1 changed")).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: "Publish" }));
		await expect(
			await canvas.findByText("Anchor didn't publish this tree"),
		).toBeVisible();
		await expect(
			await canvas.findByText("Role to fix: Project editor."),
		).toBeVisible();
		await expect(canvas.getByText("Draft: 1 changed")).toBeVisible();
	},
};

/** Anchor refuses to take away a parent type that resources still sit under. */
export const PublishErrorResourcesUnderParent: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			publish: {
				status: 409,
				body: {
					errors: [
						{
							code: "RESOURCE_TYPE_IN_USE",
							message:
								"2 registered resources of type document sit under one of type project.",
							metadata: {
								slug: "document",
								parent_type: "project",
								blocked_by: { resources: 2 },
							},
						},
					],
				},
			},
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Document"));
		await userEvent.click(
			inspector(canvasElement).getByRole("checkbox", { name: "Project" }),
		);
		await expect(canvas.getByText("Draft: 1 changed")).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: "Publish" }));
		await expect(
			await canvas.findByText(
				"Document can't lose Project as a parent type: 2 registered resources sit under a Project. Move them first.",
			),
		).toBeVisible();
		await expect(canvas.getByText("Draft: 1 changed")).toBeVisible();
	},
};

/** The + handle inserts a child in a naming state; Enter keeps it. */
export const AddWithPointer: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Folder"));
		await userEvent.click(
			canvas.getByRole("button", { name: "Add a type under Folder" }),
		);
		const input = canvas.getByRole("textbox", { name: "New type name" });
		await expect(input).toHaveFocus();
		await expect(
			canvasElement.querySelector('[data-entering="true"]'),
		).not.toBeNull();
		await expect(canvas.queryByText("Fix these before publishing")).toBeNull();
		await expect(
			inspector(canvasElement).getByText(
				"Name it in the tree. Enter keeps it, Escape drops it.",
			),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Publish" }),
		).toBeDisabled();
		await userEvent.type(input, "Section{Enter}");
		await expect(canvas.getByText("Draft: 1 type added")).toBeVisible();
		const section = await typeNode(canvasElement, "Section");
		await expect(section).toHaveFocus();
		await expect(within(section).getByText("section")).toBeVisible();
		await expect(inspector(canvasElement).getByLabelText("Slug")).toHaveValue(
			"section",
		);
	},
};

/** Keyboard edits never animate; Escape drops a new type. */
export const KeyboardEditing: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await startEditing(canvasElement);
		(await typeNode(canvasElement, "Organization")).focus();
		await userEvent.keyboard("{ArrowDown}");
		await expect(await typeNode(canvasElement, "Project")).toHaveFocus();
		await userEvent.keyboard("{ArrowDown}");
		await expect(document.activeElement?.getAttribute("aria-label")).toMatch(
			/^(Folder|Document)$/,
		);
		(await typeNode(canvasElement, "Folder")).focus();
		await userEvent.keyboard("n");
		const input = canvas.getByRole("textbox", { name: "New type name" });
		await expect(input).toHaveFocus();
		await expect(
			canvasElement.querySelector('[data-entering="true"]'),
		).toBeNull();
		await userEvent.keyboard("{Escape}");
		await expect(
			canvas.queryByRole("textbox", { name: "New type name" }),
		).toBeNull();
		await expect(await typeNode(canvasElement, "Folder")).toHaveFocus();
		await expect(canvas.getByText("Draft: No changes yet")).toBeVisible();
		await userEvent.keyboard("{Enter}");
		const rename = canvas.getByRole("textbox", { name: "Rename Folder" });
		await userEvent.clear(rename);
		await userEvent.type(rename, "Directory{Enter}");
		await expect(await typeNode(canvasElement, "Directory")).toHaveFocus();
		await expect(canvas.getByText("Draft: 1 renamed")).toBeVisible();
		await expect(
			within(await typeNode(canvasElement, "Directory")).getByText("folder"),
		).toBeVisible();
	},
};

/** Delete is disabled with the reasons while anything uses the type. */
export const DeleteBlocked: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			usage: { folder: { roles: 3, permissions: 1 } },
		},
	},
	play: async ({ canvasElement }) => {
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Folder"));
		const details = inspector(canvasElement);
		await expect(
			await details.findByText("3 roles · 1 permission"),
		).toBeVisible();
		await expect(
			details.getByRole("button", { name: "Delete Folder" }),
		).toBeDisabled();
		await expect(details.getByText("1 type sits under Folder.")).toBeVisible();
		await expect(
			details.getByText("3 roles are scoped to Folder."),
		).toBeVisible();
		await expect(
			details.getByText("1 permission is scoped to Folder."),
		).toBeVisible();
		(await typeNode(canvasElement, "Folder")).focus();
		await userEvent.keyboard("{Delete}");
		await expect(screen.queryByRole("alertdialog")).toBeNull();
	},
};

/** Publish sends the whole tree without Organization, then reads it back. */
export const DeleteAndPublish: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Document"));
		await userEvent.click(
			await inspector(canvasElement).findByRole("button", {
				name: "Delete Document",
			}),
		);
		const dialog = await screen.findByRole("alertdialog");
		await userEvent.click(
			within(dialog).getByRole("button", { name: "Delete type" }),
		);
		await expect(canvas.getByText("Draft: 1 deleted")).toBeVisible();
		await expect(await typeNode(canvasElement, "Folder")).toHaveFocus();
		await userEvent.click(canvas.getByRole("button", { name: "Publish" }));
		await waitFor(() =>
			expect(
				canvas.getByRole("button", { name: "Edit resource types" }),
			).toHaveFocus(),
		);
		await expect(published).toEqual([
			{
				resource_types: [
					{
						slug: "project",
						name: "Project",
						parent_types: ["organization"],
						nests_in_itself: false,
					},
					{
						slug: "folder",
						name: "Folder",
						parent_types: ["project"],
						nests_in_itself: true,
					},
				],
			},
		]);
		await waitFor(() =>
			expect(canvas.queryByRole("button", { name: "Document" })).toBeNull(),
		);
	},
};

/** Discard restores the published tree and leaves edit mode. */
export const Discard: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Project"));
		const name = inspector(canvasElement).getByLabelText("Name");
		await userEvent.clear(name);
		await userEvent.type(name, "Space");
		await expect(await typeNode(canvasElement, "Space")).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: "Discard" }));
		await expect(await typeNode(canvasElement, "Project")).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Edit resource types" }),
		).toBeVisible();
	},
};

function ruleFor(selector: string) {
	for (const sheet of Array.from(document.styleSheets))
		for (const rule of Array.from(sheet.cssRules))
			if (rule instanceof CSSStyleRule && rule.selectorText === selector)
				return rule.style;
	return undefined;
}

/** Positions animate in JavaScript; nodes never carry a CSS transition. */
export const MotionContract: Story = {
	play: async ({ canvasElement }) => {
		const node = await typeNode(canvasElement, "Project");
		const wrapper = node.closest(".react-flow__node") as HTMLElement;
		await expect(getComputedStyle(wrapper).transitionDuration).toBe("0s");
		await expect(getComputedStyle(node).transitionProperty).toBe(
			"border-color",
		);
		await expect(ruleFor(".resource-type-card")?.transitionDuration).toBe(
			"120ms",
		);
		await startEditing(canvasElement);
		await userEvent.click(node);
		await userEvent.click(
			within(canvasElement).getByRole("button", {
				name: "Add a type under Project",
			}),
		);
		const entering = canvasElement.querySelector(
			'[data-entering="true"]',
		) as HTMLElement;
		await expect(getComputedStyle(entering).animationName).toBe(
			"resource-type-enter",
		);
		await expect(
			ruleFor('.resource-type-card[data-entering="true"]')?.animationDuration,
		).toBe("150ms");
	},
};

/** Each type links to the roles and permissions scoped to it. */
export const LinksToScopedPages: Story = {
	play: async ({ canvasElement }) => {
		await userEvent.click(await typeNode(canvasElement, "Folder"));
		await expect(
			inspector(canvasElement).getByRole("link", { name: "View roles" }),
		).toHaveAttribute("href", "/products/resources/roles?resource_type=folder");
		await expect(
			inspector(canvasElement).getByRole("link", { name: "View permissions" }),
		).toHaveAttribute(
			"href",
			"/products/resources/permissions?resource_type=folder",
		);
		await expect(
			inspector(canvasElement).getByText("Custom roles off"),
		).toBeVisible();
	},
};

/** Organization's custom roles switch is part of the draft; publishing saves it on its own, without the tree call. */
export const CustomRolesSwitch: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Folder"));
		await expect(
			inspector(canvasElement).getByRole("switch", { name: "Custom roles" }),
		).toHaveAttribute("aria-disabled", "true");
		await userEvent.click(await typeNode(canvasElement, "Organization"));
		await userEvent.click(
			inspector(canvasElement).getByRole("switch", { name: "Custom roles" }),
		);
		await expect(
			canvas.getByText("Draft: custom roles switched on 1 type"),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: "Publish" }));
		await expect(
			await canvas.findByRole("button", { name: "Edit resource types" }),
		).toBeVisible();
		await expect(published).toEqual([
			{ patch: "rtype_organization", custom_roles: true },
		]);
		await waitFor(() =>
			expect(
				inspector(canvasElement).getByText("Custom roles on"),
			).toBeVisible(),
		);
	},
};

/** A refused switch keeps the draft and explains why. */
export const CustomRolesRefused: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			switchReply: {
				status: 409,
				body: {
					errors: [
						{
							code: "BUILT_IN_RESOURCE_TYPE",
							message: "Organization is built in.",
						},
					],
				},
			},
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await startEditing(canvasElement);
		await userEvent.click(await typeNode(canvasElement, "Organization"));
		await userEvent.click(
			inspector(canvasElement).getByRole("switch", { name: "Custom roles" }),
		);
		await userEvent.click(canvas.getByRole("button", { name: "Publish" }));
		await expect(
			await canvas.findByText("Anchor didn't save the custom roles switch"),
		).toBeVisible();
		await expect(
			canvas.getByText("Draft: custom roles switched on 1 type"),
		).toBeVisible();
	},
};
