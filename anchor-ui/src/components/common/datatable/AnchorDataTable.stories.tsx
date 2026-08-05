import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ColumnDef } from "@tanstack/react-table";
import { expect, screen, userEvent, within } from "storybook/test";

import { StatusBadge } from "@/components/common/StatusBadge";

import { AnchorDataTable } from "./AnchorDataTable";

type ApiKey = {
	id: string;
	name: string;
	status: string;
	lastUsed: string;
};

const columns: ColumnDef<ApiKey, string>[] = [
	{ accessorKey: "name", header: "Name" },
	{
		accessorKey: "status",
		header: "Status",
		cell: ({ row }) => <StatusBadge>{row.original.status}</StatusBadge>,
	},
	{ accessorKey: "lastUsed", header: "Last used" },
];

const rows: ApiKey[] = [
	{
		id: "key_01",
		name: "checkout-prod",
		status: "ACTIVE",
		lastUsed: "2 minutes ago",
	},
	{
		id: "key_02",
		name: "checkout-staging",
		status: "ACTIVE",
		lastUsed: "3 days ago",
	},
	{ id: "key_03", name: "legacy-import", status: "REVOKED", lastUsed: "never" },
];

const meta = {
	title: "Common/AnchorDataTable",
	component: AnchorDataTable,
	tags: ["autodocs"],
	args: {
		columns,
		data: rows,
		total: rows.length,
		pagination: { pageIndex: 0, pageSize: 10 },
		onPaginationChange: () => {},
		sorting: [],
		onSortingChange: () => {},
	},
	parameters: {
		layout: "padded",
	},
} satisfies Meta<typeof AnchorDataTable<ApiKey>>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("columnheader", { name: "Name" }),
		).toBeInTheDocument();
		await expect(canvas.getByText("checkout-prod")).toBeInTheDocument();
		await expect(canvas.getByText("legacy-import")).toBeInTheDocument();
	},
};

/**
 * The loading state replaces rows with skeletons. Asserting "no data leaked
 * through" matters more than the skeleton count — a table that shows stale rows
 * while refetching reads as fresh data to the user.
 */
export const Loading: Story = {
	args: {
		loading: true,
		data: [],
		total: 0,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.queryByText("checkout-prod")).not.toBeInTheDocument();
		await expect(canvas.getByRole("table")).toBeInTheDocument();
	},
};

export const Empty: Story = {
	args: {
		data: [],
		total: 0,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("No results.")).toBeInTheDocument();
	},
};

/**
 * KNOWN DEFECT — the header select control is inert.
 *
 * It is a `DropdownMenuTrigger` whose `render` prop is a `Checkbox`. Clicking it
 * does NOTHING: no Select menu opens, and `onCheckedChange` never fires, so
 * `selectAllMode` never leaves "none". Both behaviours are lost in the
 * composition — the same family of Base UI `render`-prop trap as the
 * `Menu.GroupLabel` error #31 case, and like that one the fix belongs at this
 * call site, not in `components/ui/dropdown-menu.tsx`.
 *
 * The assertions below pin the CURRENT broken behaviour so the fix has a
 * failing baseline. When the control works, invert them: the menu opens, "All
 * in current page" selects every visible row.
 */
export const SelectAllIsInert: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(canvas.getByLabelText("Select all"));

		await expect(
			screen.queryByRole("menuitem", { name: "All in current page" }),
		).not.toBeInTheDocument();

		for (const box of canvas.getAllByLabelText("Select row")) {
			await expect(box).not.toBeChecked();
		}
	},
};

/**
 * KNOWN DEFECT — selecting ONE row does not stick.
 *
 * The row checkbox's `onCheckedChange` calls `row.toggleSelected(value)` and
 * then `setSelectAllMode("none")`; the effect watching `selectAllMode` responds
 * to "none" by clearing `rowSelection` outright, so the toggle is wiped on the
 * next render. `onSelectionChange` fires with `[]` and the box stays unchecked.
 *
 * Same deal as above: the assertion pins the bug so the fix breaks it loudly.
 */
export const RowSelectionDoesNotStick: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const [firstRow] = canvas.getAllByLabelText("Select row");

		await userEvent.click(firstRow);

		await expect(firstRow).not.toBeChecked();
	},
};

export const SelectionDisabled: Story = {
	args: {
		enableRowSelection: false,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.queryAllByRole("checkbox")).toHaveLength(0);
	},
};

/**
 * Full-text search is controlled by the caller — the table only reports what
 * was typed and never filters locally, so the route stays the single source of
 * truth for what the server was asked for.
 */
export const WithSearch: Story = {
	args: {
		fullTextSearch: "",
		fullTextSearchPlaceHolder: "Search keys...",
		onFullTextSearchChange: () => {},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByPlaceholderText("Search keys..."),
		).toBeInTheDocument();
	},
};
