import { type LicenseTemplateResponse, LicenseTemplateStatus } from "@/client";
import {
	getLicenseSchemaOptions,
	listLicenseTemplatesOptions,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { Text } from "@nanostackorg/design-system/components/text";
import { TextLink } from "@nanostackorg/design-system/components/text-link";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import {
	EyeIcon as Eye,
	LayoutIcon as LayoutTemplate,
	PencilLineIcon as PenLine,
	PlusIcon as Plus,
} from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { useMemo, useState } from "react";

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
	const navigate = useNavigate();
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
				cell: (info) => (
					<TextLink
						href={ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_DETAIL.replace(
							"$templateId",
							encodeURIComponent(info.row.original.id),
						)}
					>
						{info.getValue()}
					</TextLink>
				),
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
					<Box
						as="span"
						className="block max-w-[220px] truncate text-sm text-muted-foreground"
					>
						{info.getValue() || "No description"}
					</Box>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("values", {
				header: () => <span>Values</span>,
				cell: (info) => (
					<Text as="span" tone="muted">
						{Object.keys(info.getValue() ?? {}).length} field
						{Object.keys(info.getValue() ?? {}).length === 1 ? "" : "s"}
					</Text>
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
					<Inline space="sm" wrap={false}>
						<ButtonLink
							href={ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_DETAIL.replace(
								"$templateId",
								encodeURIComponent(row.original.id),
							)}
							variant="outline"
							icon={Eye}
							aria-label={`View ${row.original.name}`}
						>
							View
						</ButtonLink>
						{row.original.status === LicenseTemplateStatus.ACTIVE && (
							<ButtonLink
								href={`${ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_DETAIL.replace("$templateId", encodeURIComponent(row.original.id))}?edit=true`}
								variant="outline"
								icon={PenLine}
								aria-label={`Edit ${row.original.name}`}
							>
								Edit
							</ButtonLink>
						)}
					</Inline>
				),
			}),
		],
		[],
	);

	if (!schemaQuery.isLoading && !schema) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia icon={LayoutTemplate} />
					<EmptyTitle>No license schema declared yet</EmptyTitle>
					<EmptyDescription>
						A template is a set of values checked against this product&rsquo;s
						license schema. Declare the schema first.
					</EmptyDescription>
				</EmptyHeader>
				<ButtonLink href={ROUTE_PATHS.PRODUCT_LICENSE_SCHEMA} variant="outline">
					Go to License Schema
				</ButtonLink>
			</Empty>
		);
	}

	return (
		<>
			<Box className="mb-4 flex items-center justify-between">
				<div />
				{schema && (
					<ButtonLink
						href={ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_NEW}
						variant="solid"
						tone="brand"
						icon={Plus}
					>
						Create Template
					</ButtonLink>
				)}
			</Box>
			<AnchorDataTable
				columns={columns}
				data={pageItems}
				onRowClick={(template) => {
					void navigate({
						to: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_DETAIL,
						params: { templateId: template.id },
						search: { edit: true },
					});
				}}
				isRowClickable={(template) =>
					!!schema && template.status === LicenseTemplateStatus.ACTIVE
				}
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
