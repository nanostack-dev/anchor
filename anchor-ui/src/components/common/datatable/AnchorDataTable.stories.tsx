import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ColumnDef } from "@tanstack/react-table";
import { expect, fn, screen, userEvent, within } from "storybook/test";

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

/**
 * Nothing exists yet. Distinct from the filtered-empty story below: this one
 * offers no "clear" action, because there is nothing applied to clear.
 */
export const Empty: Story = {
	args: {
		data: [],
		total: 0,
		resourceName: "API keys",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("No API keys yet")).toBeInTheDocument();
		await expect(
			canvas.queryByRole("button", { name: /^Clear/ }),
		).not.toBeInTheDocument();
	},
};

/**
 * Empty *because the user narrowed the view*. The copy names what was applied
 * and the recovery button performs exactly that — clearing the search puts the
 * user back where results exist.
 */
export const EmptyAfterSearch: Story = {
	args: {
		data: [],
		total: 0,
		resourceName: "API keys",
		fullTextSearch: "zzzz",
		onFullTextSearchChange: fn(),
	},
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("No API keys match your search"),
		).toBeInTheDocument();
		await userEvent.click(canvas.getByRole("button", { name: "Clear search" }));
		await expect(args.onFullTextSearchChange).toHaveBeenCalledWith("");
	},
};

/**
 * A failed load with nothing to fall back on. The point of the state is that it
 * cannot be mistaken for an empty list, so assert both halves: the failure is
 * named, and the "you have none" copy is absent.
 */
export const LoadError: Story = {
	args: {
		data: [],
		total: 0,
		resourceName: "API keys",
		error: new Error("api-keys service unavailable"),
		onRetry: fn(),
	},
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("Couldn’t load API keys"),
		).toBeInTheDocument();
		await expect(
			canvas.getByText("api-keys service unavailable."),
		).toBeInTheDocument();
		await expect(canvas.queryByText("No API keys yet")).not.toBeInTheDocument();
		await userEvent.click(canvas.getByRole("button", { name: "Try again" }));
		await expect(args.onRetry).toHaveBeenCalled();
	},
};

/**
 * A refetch failed while previously-loaded rows are still on screen. Those rows
 * are the most valuable thing here, so they stay; the failure is reported
 * alongside them rather than replacing them.
 */
export const RefreshErrorKeepsRows: Story = {
	args: {
		resourceName: "API keys",
		error: new Error("Failed to fetch"),
		onRetry: fn(),
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("checkout-prod")).toBeInTheDocument();
		await expect(
			canvas.getByText(/Couldn’t refresh API keys/),
		).toBeInTheDocument();
		await expect(
			canvas.queryByText("Couldn’t load API keys"),
		).not.toBeInTheDocument();
	},
};

/** Reads the tick state of every row checkbox, in row order. */
const selectionState = (canvasElement: HTMLElement) =>
	within(canvasElement)
		.getAllByLabelText("Select row")
		.map((box) => box.getAttribute("aria-checked"));

/**
 * The header checkbox and the header menu are two controls, not one.
 *
 * They used to be a single `DropdownMenuTrigger` rendered *as* the `Checkbox`,
 * and neither half worked. This pins the checkbox half: ticking it selects every
 * row on the page, unticking it clears them. What blocked that was not the
 * merged trigger but the header cell around it — an unsortable column was
 * wrapped in a `disabled` button, and a disabled button eats clicks meant for
 * anything inside it.
 */
export const SelectAllTogglesPage: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(canvas.getByLabelText("Select all"));
		await expect(selectionState(canvasElement)).toEqual([
			"true",
			"true",
			"true",
		]);

		await userEvent.click(canvas.getByLabelText("Select all"));
		await expect(selectionState(canvasElement)).toEqual([
			"false",
			"false",
			"false",
		]);
	},
};

/**
 * And this pins the menu half — the scope choices the checkbox alone cannot
 * express. THIS is what the merged trigger/checkbox broke: a `Menu.Trigger`
 * rendered as a `Checkbox` never opens its menu, clickable or not. It needs a
 * trigger of its own.
 */
export const SelectAllMenuOpens: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(canvas.getByLabelText("Selection options"));

		await userEvent.click(
			await screen.findByRole("menuitem", { name: "All in current page" }),
		);

		await expect(selectionState(canvasElement)).toEqual([
			"true",
			"true",
			"true",
		]);
	},
};

/**
 * Selecting ONE row sticks, and gets reported.
 *
 * The row checkbox used to call `row.toggleSelected(value)` and then
 * `setSelectAllMode("none")`. From a cleared table that was a no-op write —
 * React bails out of a same-value `setState`, the effect never re-ran, and the
 * tick survived by accident. What did NOT survive is the report: the caller was
 * only ever told about a selection by that effect, so an individually picked row
 * left `onSelectionChange` stuck on its mount-time `[]`. The `toggleSelected`
 * assertion is a regression guard; the `onSelectionChange` one is the defect.
 */
export const RowSelectionSticks: Story = {
	args: {
		onSelectionChange: fn(),
	},
	play: async ({ args, canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(canvas.getAllByLabelText("Select row")[0]);

		await expect(selectionState(canvasElement)).toEqual([
			"true",
			"false",
			"false",
		]);
		await expect(canvas.getByText(/1 of 3 row\(s\) selected/)).toBeVisible();
		await expect(args.onSelectionChange).toHaveBeenLastCalledWith([rows[0]]);
	},
};

/**
 * Deselecting one row out of a whole-page selection keeps the rest. This is the
 * case the old "none"-means-both state machine could not express at all: the
 * moment any row checkbox moved, every row was cleared.
 */
export const RowDeselectionKeepsTheRest: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(canvas.getByLabelText("Select all"));
		await userEvent.click(canvas.getAllByLabelText("Select row")[1]);

		await expect(selectionState(canvasElement)).toEqual([
			"true",
			"false",
			"true",
		]);
		await expect(canvas.getByText(/2 of 3 row\(s\) selected/)).toBeVisible();
	},
};

/**
 * Cells survive a selection change instead of being remounted.
 *
 * The column array handed to `useReactTable` used to be rebuilt inline on every
 * render, so `flexRender` handed React a brand new component type each time and
 * every header and cell subtree unmounted — on each selection toggle, and on
 * each keystroke in the search box. A node captured before a click ended up
 * detached, still reading `aria-checked="false"` while the live table showed the
 * row selected. Holding the reference across the click is the assertion: it only
 * passes while the column definitions stay referentially stable.
 */
export const SelectionDoesNotRemountCells: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		const firstRowCheckbox = canvas.getAllByLabelText("Select row")[0];
		await userEvent.click(firstRowCheckbox);

		await expect(firstRowCheckbox).toBeInTheDocument();
		await expect(firstRowCheckbox).toHaveAttribute("aria-checked", "true");
		await expect(canvas.getAllByLabelText("Select row")[0]).toBe(
			firstRowCheckbox,
		);
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
