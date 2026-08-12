import { type LicenseTemplateResponse, LicenseTemplateStatus } from "@/client";
import {
	getLicenseSchemaOptions,
	listLicenseTemplatesOptions,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { Eye, LayoutTemplate, PenLine, Plus } from "lucide-react";
import { useMemo, useState } from "react";
import { LicenseTemplateFormDialog } from "./LicenseTemplateFormDialog";
import { LicenseTemplateViewDialog } from "./LicenseTemplateViewDialog";

const columnHelper = createColumnHelper<LicenseTemplateResponse>();

const STATUS_OPTIONS = [
	{ label: "Active", value: LicenseTemplateStatus.ACTIVE },
	{ label: "Archived", value: LicenseTemplateStatus.ARCHIVED },
];

interface LicenseTemplateDatatableProps {
	productId: string;
}

/**
 * Lists a product's license templates. The list route returns every template
 * unpaginated (ordered by name — see its OpenAPI description), so paging,
 * search and the status filter all run client-side over that one fetch
 * rather than round-tripping the API on every keystroke.
 */
export function LicenseTemplateDatatable({
	productId,
}: LicenseTemplateDatatableProps) {
	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([]);
	const [fullTextSearch, setFullTextSearch] = useState("");
	const debouncedSearch = useDebounce(fullTextSearch, 300);
	const [statusFilter, setStatusFilter] = useState<string[]>([]);

	const schemaQuery = useQuery({
		...getLicenseSchemaOptions({ path: { product_id: productId } }),
		retry: false,
	});
	const schema = schemaQuery.data;

	const templatesQuery = useQuery({
		...listLicenseTemplatesOptions({ path: { product_id: productId } }),
	});
	const allItems = templatesQuery.data?.items ?? [];

	const filtered = useMemo(() => {
		let items = allItems;
		if (statusFilter.length > 0) {
			items = items.filter((t) => statusFilter.includes(t.status));
		}
		if (debouncedSearch) {
			const q = debouncedSearch.toLowerCase();
			items = items.filter(
				(t) =>
					t.name.toLowerCase().includes(q) ||
					(t.description ?? "").toLowerCase().includes(q),
			);
		}
		const sort = sorting[0];
		if (sort) {
			const { id, desc } = sort;
			items = [...items].sort((a, b) => {
				const av = String(a[id as keyof LicenseTemplateResponse] ?? "");
				const bv = String(b[id as keyof LicenseTemplateResponse] ?? "");
				const cmp = av.localeCompare(bv);
				return desc ? -cmp : cmp;
			});
		}
		return items;
	}, [allItems, statusFilter, debouncedSearch, sorting]);

	const pageItems = useMemo(() => {
		const start = pagination.pageIndex * pagination.pageSize;
		return filtered.slice(start, start + pagination.pageSize);
	}, [filtered, pagination]);

	const columns = useMemo(
		() => [
			columnHelper.accessor("name", {
				header: () => <span>Name</span>,
				cell: (info) => <span className="font-medium">{info.getValue()}</span>,
				enableSorting: true,
			}),
			columnHelper.accessor("status", {
				header: () => <span>Status</span>,
				cell: (info) => (
					<StatusBadge
						tone={
							info.getValue() === LicenseTemplateStatus.ACTIVE
								? "success"
								: "neutral"
						}
					>
						{info.getValue()}
					</StatusBadge>
				),
				enableSorting: true,
			}),
			columnHelper.accessor("description", {
				header: () => <span>Description</span>,
				cell: (info) => (
					<span className="block max-w-[220px] truncate text-sm text-muted-foreground">
						{info.getValue() || "No description"}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("values", {
				header: () => <span>Values</span>,
				cell: (info) => (
					<span className="text-sm text-muted-foreground">
						{Object.keys(info.getValue() ?? {}).length} field
						{Object.keys(info.getValue() ?? {}).length === 1 ? "" : "s"}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("created_at", {
				header: () => <span>Created At</span>,
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
				enableSorting: true,
			}),
			columnHelper.display({
				id: "actions",
				header: () => <span>Actions</span>,
				cell: ({ row }) => (
					<div className="flex gap-2">
						{schema && (
							<LicenseTemplateViewDialog
								template={row.original}
								fields={schema.fields}
								trigger={
									<Button variant="outline" size="icon">
										<span className="sr-only">View template</span>
										<Eye className="h-4 w-4" />
									</Button>
								}
							/>
						)}
						{schema && row.original.status === LicenseTemplateStatus.ACTIVE && (
							<LicenseTemplateFormDialog
								productId={productId}
								schema={schema}
								mode="edit"
								existingTemplate={row.original}
								trigger={
									<Button variant="outline" size="icon">
										<span className="sr-only">Edit template</span>
										<PenLine className="h-4 w-4" />
									</Button>
								}
							/>
						)}
					</div>
				),
			}),
		],
		[productId, schema],
	);

	if (!schemaQuery.isLoading && !schema) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<LayoutTemplate />
					</EmptyMedia>
					<EmptyTitle>No license schema declared yet</EmptyTitle>
					<EmptyDescription>
						A template is a set of values checked against this product&rsquo;s
						license schema. Declare the schema first.
					</EmptyDescription>
				</EmptyHeader>
				<Button
					variant="outline"
					render={<Link to={ROUTE_PATHS.PRODUCT_LICENSE_SCHEMA} />}
				>
					Go to License Schema
				</Button>
			</Empty>
		);
	}

	return (
		<>
			<div className="mb-4 flex items-center justify-between">
				<div />
				{schema && (
					<LicenseTemplateFormDialog
						productId={productId}
						schema={schema}
						mode="create"
						trigger={
							<Button>
								<Plus />
								Create Template
							</Button>
						}
					/>
				)}
			</div>
			<AnchorDataTable
				columns={columns}
				data={pageItems}
				loading={templatesQuery.isLoading || schemaQuery.isLoading}
				resourceName="license templates"
				error={templatesQuery.error}
				onRetry={() => void templatesQuery.refetch()}
				total={filtered.length}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				fullTextSearch={fullTextSearch}
				onFullTextSearchChange={setFullTextSearch}
				fullTextSearchPlaceHolder="Search templates"
				filters={[
					{
						key: "status",
						label: "Status",
						type: "select",
						value: statusFilter,
						options: STATUS_OPTIONS,
						placeholder: "Filter by status",
						multi: true,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setStatusFilter(Array.isArray(filters.status) ? filters.status : []);
				}}
				enableRowSelection={false}
			/>
		</>
	);
}
