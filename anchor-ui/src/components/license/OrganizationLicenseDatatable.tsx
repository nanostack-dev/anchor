import {
	type LicenseTemplateResponse,
	type OrganizationLicenseSearchRequest,
	OrganizationLicenseSortField,
	type OrganizationLicenseSummaryResponse,
	SortDirection,
	searchOrganizationLicenses,
} from "@/client";
import {
	getLicenseSchemaOptions,
	listLicenseTemplatesOptions,
	searchOrganizationLicensesOptions,
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
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { ArrowRightLeft, Eye, ScrollText } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { toast } from "sonner";
import { LicenseMigrationDialog } from "./LicenseMigrationDialog";
import { differsFromItsTemplate } from "./license-migration-format";

const columnHelper = createColumnHelper<OrganizationLicenseSummaryResponse>();

/**
 * The cap Anchor puts on one migration run. Selecting every organization
 * matching a query resolves to explicit identifiers, so the same ceiling
 * applies here and is refused with the same number rather than a truncated set.
 */
const MIGRATION_LIMIT = 500;

interface OrganizationLicenseDatatableProps {
	productId: string;
}

/**
 * The product's customer book: every organization and the tier it is on.
 *
 * Paging, search and the tier filter are server-side — the point of the list is
 * to select a cohort out of all of them, so filtering a single fetched page
 * would answer the wrong question.
 */
export function OrganizationLicenseDatatable({
	productId,
}: OrganizationLicenseDatatableProps) {
	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 20,
	});
	const [sorting, setSorting] = useState<SortingState>([]);
	const [fullTextSearch, setFullTextSearch] = useState("");
	const debouncedSearch = useDebounce(fullTextSearch, 300);
	const [templateFilter, setTemplateFilter] = useState<string[]>([]);
	const [selection, setSelection] = useState<
		OrganizationLicenseSummaryResponse[]
	>([]);
	const [allMatching, setAllMatching] = useState(false);
	const [migrateOpen, setMigrateOpen] = useState(false);
	const [resolvingSelection, setResolvingSelection] = useState(false);

	const schemaQuery = useQuery({
		...getLicenseSchemaOptions({ path: { product_id: productId } }),
		retry: false,
	});
	const schema = schemaQuery.data;

	const templatesQuery = useQuery(
		listLicenseTemplatesOptions({ path: { product_id: productId } }),
	);
	const templates = useMemo(
		() => templatesQuery.data?.items ?? [],
		[templatesQuery.data],
	);
	const templatesById = useMemo(
		() => new Map(templates.map((template) => [template.id, template])),
		[templates],
	);

	const searchBody = useMemo(
		() => ({
			pagination: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
			// The table sorts server-side, so the header's state has to reach the
			// route or the arrow moves and the rows do not.
			sort_by: mapSortingToApiField<
				OrganizationLicenseSearchRequest["sort_by"]
			>(sorting[0]?.id, OrganizationLicenseSortField.ORGANIZATION_NAME),
			sort_direction: sorting[0]?.desc ? SortDirection.DESC : SortDirection.ASC,
			full_text_search: debouncedSearch || undefined,
			filter:
				templateFilter.length > 0
					? { license_template_ids: templateFilter }
					: undefined,
		}),
		[pagination, sorting, debouncedSearch, templateFilter],
	);

	const licensesQuery = useQuery(
		searchOrganizationLicensesOptions({
			path: { product_id: productId },
			body: searchBody,
		}),
	);
	// Referentially stable: AnchorDataTable's selection effect lists `data` and
	// `onSelectionChange` among its dependencies, so a fresh array or a fresh
	// callback on every render drives it into an update loop.
	const items = useMemo(
		() => licensesQuery.data?.items ?? [],
		[licensesQuery.data],
	);

	const matchingTotal = licensesQuery.data?.total;
	const matchingLabel =
		allMatching && matchingTotal !== undefined
			? `all ${matchingTotal}`
			: "all matching";

	const templateOptions = useMemo(
		() =>
			templates.map((template) => ({
				label:
					template.status === "ARCHIVED"
						? `${template.name} (archived)`
						: template.name,
				value: template.id,
			})),
		[templates],
	);

	/**
	 * "All matching query" is the reason this list exists — a cohort is usually
	 * larger than a page. The migration route takes explicit identifiers, so the
	 * whole matching set is fetched once here rather than paged through.
	 */
	const resolveAllMatching = async () => {
		setResolvingSelection(true);
		try {
			const { data, error } = await searchOrganizationLicenses({
				path: { product_id: productId },
				body: {
					...searchBody,
					pagination: { limit: MIGRATION_LIMIT + 1, offset: 0 },
				},
			});
			if (error || !data) {
				toast.error("Could not read the organizations matching this query.");
				return;
			}
			if (data.items.length > MIGRATION_LIMIT) {
				toast.error(
					`${data.total} organizations match. At most ${MIGRATION_LIMIT} can move in one run — narrow the search first.`,
				);
				return;
			}
			setSelection(data.items);
			setMigrateOpen(true);
		} finally {
			setResolvingSelection(false);
		}
	};

	// "all-matching" carries no rows, so it is kept as its own state rather
	// than flattened to an empty selection: an operator who ticked every match
	// and one who ticked nothing must not read the same button.
	const handleSelectionChange = useCallback(
		(selected: OrganizationLicenseSummaryResponse[] | "all-matching" | []) => {
			setAllMatching(selected === "all-matching");
			setSelection(selected === "all-matching" ? [] : selected);
		},
		[],
	);

	const columns = useMemo(
		() => [
			columnHelper.accessor("organization_name", {
				header: () => <span>Organization</span>,
				cell: (info) => <span className="font-medium">{info.getValue()}</span>,
				enableSorting: true,
			}),
			columnHelper.display({
				id: "tier",
				header: () => <span>Tier</span>,
				cell: ({ row }) => {
					const template = row.original.license
						? templatesById.get(row.original.license.template_id)
						: undefined;
					if (!row.original.license) {
						return <StatusBadge tone="neutral">No license</StatusBadge>;
					}
					return (
						<div className="flex items-center gap-2">
							<span className="font-medium">
								{template?.name ?? row.original.license.template_id}
							</span>
							{template?.status === "ARCHIVED" && (
								<StatusBadge tone="warning">Archived</StatusBadge>
							)}
						</div>
					);
				},
			}),
			columnHelper.display({
				id: "differs",
				header: () => <span>Matches tier</span>,
				cell: ({ row }) => {
					if (!row.original.license)
						return <span className="text-muted-foreground">—</span>;
					const template = templatesById.get(row.original.license.template_id);
					const differs = differsFromItsTemplate(
						row.original,
						template?.values,
					);
					if (differs === undefined) {
						return <span className="text-muted-foreground">—</span>;
					}
					return differs ? (
						<StatusBadge tone="info">Adjusted</StatusBadge>
					) : (
						<span className="text-sm text-muted-foreground">Matches</span>
					);
				},
			}),
			columnHelper.display({
				id: "instantiated_at",
				header: () => <span>On tier since</span>,
				cell: ({ row }) =>
					row.original.license ? (
						dayjs(row.original.license.instantiated_at).format("D MMMM YYYY")
					) : (
						<span className="text-muted-foreground">—</span>
					),
			}),
			columnHelper.display({
				id: "actions",
				header: () => <span>Actions</span>,
				cell: ({ row }) => (
					<Button
						variant="outline"
						size="icon"
						nativeButton={false}
						render={
							<Link
								to={ROUTE_PATHS.ORGANIZATION_LICENSE_DETAIL}
								params={{ organizationId: row.original.organization_id }}
							/>
						}
					>
						<span className="sr-only">
							Open {row.original.organization_name}&rsquo;s license
						</span>
						<Eye className="size-4" />
					</Button>
				),
			}),
		],
		[templatesById],
	);

	if (!schemaQuery.isLoading && !schema) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<ScrollText />
					</EmptyMedia>
					<EmptyTitle>No license schema declared yet</EmptyTitle>
					<EmptyDescription>
						An organization&rsquo;s license is a copy of a template&rsquo;s
						values, and a template is checked against this product&rsquo;s
						schema. Declare the schema first.
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
			<AnchorDataTable
				columns={columns}
				data={items}
				loading={licensesQuery.isLoading || schemaQuery.isLoading}
				resourceName="organizations"
				error={licensesQuery.error}
				onRetry={() => void licensesQuery.refetch()}
				total={licensesQuery.data?.total ?? 0}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				fullTextSearch={fullTextSearch}
				onFullTextSearchChange={(value) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setFullTextSearch(value);
				}}
				fullTextSearchPlaceHolder="Search organizations"
				filters={[
					{
						key: "template",
						label: "Tier",
						type: "select",
						value: templateFilter,
						options: templateOptions,
						placeholder: "Filter by tier",
						multi: true,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setTemplateFilter(
						Array.isArray(filters.template) ? filters.template : [],
					);
				}}
				onSelectionChange={handleSelectionChange}
			>
				<Button
					variant="outline"
					size="sm"
					disabled={resolvingSelection || templatesQuery.isLoading || !schema}
					onClick={() => {
						if (selection.length > 0) {
							setMigrateOpen(true);
							return;
						}
						void resolveAllMatching();
					}}
				>
					<ArrowRightLeft />
					<span className="hidden sm:inline">
						{selection.length > 0
							? `Move ${selection.length} to another tier`
							: `Move ${matchingLabel} to another tier`}
					</span>
					<span className="sm:hidden">
						{selection.length > 0
							? `Move ${selection.length}`
							: `Move ${matchingLabel}`}
					</span>
				</Button>
			</AnchorDataTable>

			{schema && (
				<LicenseMigrationDialog
					open={migrateOpen}
					onOpenChange={setMigrateOpen}
					productId={productId}
					selection={selection}
					templates={templates as LicenseTemplateResponse[]}
					fields={schema.fields}
					onMigrated={() => setSelection([])}
				/>
			)}
		</>
	);
}
