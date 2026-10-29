import type { ProductResourceTypeResponse } from "@/client";
import { treeOrder } from "@/components/product/resource-types/resource-type-scope";
import {
	DEEP_TREE,
	DOCUMENT_PERMISSIONS,
	DOCUMENT_TREE,
	FIFTY_ONE_TYPES,
	LONG_NAME,
	storyPermission,
	storyType,
} from "@/lib/storybook/access-backend";
import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import {
	type PermissionListState,
	ScopedPermissionPicker,
} from "./ScopedPermissionPicker";

interface HarnessProps {
	tree: ProductResourceTypeResponse[];
	scope: string;
	state: PermissionListState;
	initial?: string[];
	onChange?: (selected: string[]) => void;
}

function Harness({ tree, scope, state, initial = [], onChange }: HarnessProps) {
	const [selected, setSelected] = useState(initial);
	return (
		<StoryRouter>
			<div style={{ maxWidth: 720 }}>
				<ScopedPermissionPicker
					types={treeOrder(tree)}
					scope={scope}
					state={state}
					selected={selected}
					onChange={(next) => {
						setSelected(next);
						onChange?.(next);
					}}
					newPermissionHref={`/products/resources/new-permission?resource_type=${scope}`}
				/>
			</div>
		</StoryRouter>
	);
}

const ready = (permissions = DOCUMENT_PERMISSIONS): PermissionListState => ({
	status: "ready",
	permissions,
});

const meta = {
	title: "Product/ScopedPermissionPicker",
	component: Harness,
	parameters: { layout: "padded" },
	args: {
		tree: DOCUMENT_TREE,
		scope: "project",
		state: ready(),
		onChange: fn(),
	},
} satisfies Meta<typeof Harness>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A project role sees project, folder and document permissions, grouped top down; Organization's are not offered. */
export const ProjectScope: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const headings = canvas.getAllByRole("heading", { level: 3 });
		await expect(headings.map((heading) => heading.textContent)).toEqual([
			"Project",
			"Folder",
			"Document",
		]);
		await expect(
			canvas.queryByRole("checkbox", { name: "billing:read" }),
		).toBeNull();
		await expect(canvas.getByText(/Organization isn't offered/)).toBeVisible();
	},
};

/** Selecting a whole group, then searching, keeps the count honest. */
export const SelectGroupAndSearch: Story = {
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			canvas.getByRole("button", { name: "Select all Document permissions" }),
		);
		await expect(args.onChange).toHaveBeenLastCalledWith([
			"document:view",
			"document:edit",
			"document:share",
		]);
		await expect(canvas.getByText("3 of 6 selected")).toBeVisible();
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Search permissions" }),
			"folder",
		);
		await expect(canvas.getAllByRole("heading", { level: 3 })).toHaveLength(1);
		await userEvent.clear(
			canvas.getByRole("textbox", { name: "Search permissions" }),
		);
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Search permissions" }),
			"nothing-matches",
		);
		await expect(canvas.getByText(/No permission matches/)).toBeVisible();
	},
};

/** Organization covers every type, so every group is offered and nothing is left out. */
export const OrganizationScope: Story = {
	args: { scope: "organization", initial: ["members:update"] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getAllByRole("heading", { level: 3 })).toHaveLength(4);
		await expect(canvas.queryByText(/offered:/)).toBeNull();
		await expect(
			canvas.getByRole("checkbox", { name: "members:update" }),
		).toBeChecked();
	},
};

/** A leaf type offers only itself. */
export const LeafScope: Story = {
	args: { scope: "document" },
	play: async ({ canvasElement }) => {
		await expect(
			within(canvasElement).getByText(
				/can hold the permissions of that type only/,
			),
		).toBeVisible();
	},
};

/** The largest catalog: Organization over fifty types, with two permissions on ten of them. */
export const FiftyOneTypes: Story = {
	args: {
		tree: FIFTY_ONE_TYPES,
		scope: "organization",
		state: ready(
			Array.from({ length: 10 }, (_, index) => [
				storyPermission(`type_${index}:read`, `type_${index}`),
				storyPermission(`type_${index}:write`, `type_${index}`),
			]).flat(),
		),
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getAllByRole("heading", { level: 3 })).toHaveLength(10);
		await expect(
			canvas.getByText(
				/No permissions scoped to Organization, Resource type number 10/,
			),
		).toBeVisible();
	},
};

/** A deep chain: a role on the second level reaches four levels of permissions. */
export const DeepTree: Story = {
	args: {
		tree: DEEP_TREE,
		scope: "site",
		state: ready([
			storyPermission("region:audit", "region"),
			storyPermission("site:manage", "site"),
			storyPermission("building:open", "building"),
			storyPermission("room:book", "room"),
		]),
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getAllByRole("heading", { level: 3 }).map((h) => h.textContent),
		).toEqual(["Site", "Building", "Room"]);
		await expect(
			canvas.getByText(/Organization, Region aren't offered/),
		).toBeVisible();
	},
};

/** Unbroken names and descriptions wrap instead of widening the page. */
export const LongNames: Story = {
	args: {
		tree: [
			storyType("organization", "Organization"),
			storyType("evidence", LONG_NAME),
		],
		scope: "evidence",
		state: ready([
			storyPermission(
				`evidence:${LONG_NAME.toLowerCase()}`,
				"evidence",
				`${LONG_NAME} ${LONG_NAME}`,
			),
		]),
	},
	play: async ({ canvasElement }) => {
		const section = canvasElement.querySelector("section");
		await expect(section?.scrollWidth).toBeLessThanOrEqual(
			(section?.clientWidth ?? 0) + 1,
		);
	},
};

/** Nothing is scoped at or below the role's type yet: the role can still be created, and a link creates a permission there. */
export const EmptyScope: Story = {
	args: { scope: "folder", state: ready(DOCUMENT_PERMISSIONS.slice(0, 3)) },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("No permissions to add")).toBeVisible();
		await expect(
			canvas.getByRole("link", { name: "Create a permission" }),
		).toHaveAttribute(
			"href",
			"/products/resources/new-permission?resource_type=folder",
		);
	},
};

/** An older role holding a permission outside its scope learns that saving drops it. */
export const OutsideScopeSelected: Story = {
	args: { scope: "folder", initial: ["billing:read", "folder:view"] },
	play: async ({ canvasElement }) => {
		await expect(
			within(canvasElement).getByText(
				/billing:read is scoped above or beside Folder/,
			),
		).toBeVisible();
	},
};

export const Loading: Story = {
	args: { state: { status: "loading" } },
	play: async ({ canvasElement }) => {
		await expect(
			within(canvasElement).getByRole("status", {
				name: "Loading permissions",
			}),
		).toBeVisible();
	},
};

export const LoadFails: Story = {
	args: { state: { status: "error", retry: fn() } },
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Try again" }));
		await expect(
			(args.state as { retry: () => void }).retry,
		).toHaveBeenCalled();
	},
};
