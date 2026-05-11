import { useMemo, useState } from "react";
import dayjs from "dayjs";
import {
	keepPreviousData,
	useMutation,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { Link, useNavigate } from "@tanstack/react-router";
import {
	createEmailTemplateMutation,
	deleteEmailTemplateMutation,
	listEmailTemplatesOptions,
} from "@/client/@tanstack/react-query.gen";
import type {
	EmailTemplateResponse,
	ListEmailTemplatesData,
	Options,
} from "@/client";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import { useProduct } from "@/context/product/ProductContext";
import { Button } from "@/components/ui/button";
import { ROUTE_PATHS } from "@/routes/routePaths";

const columnHelper = createColumnHelper<EmailTemplateResponse>();

function uniqueSlug() {
	return `template-${Date.now().toString(36)}`;
}

export function EmailTemplatesDatatable() {
	const { currentProduct } = useProduct();
	const queryClient = useQueryClient();
	const navigate = useNavigate();

	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 20,
	});
	const [sorting, setSorting] = useState<SortingState>([]);
	const [createError, setCreateError] = useState<string | null>(null);

	const queryOptions = useMemo((): Options<ListEmailTemplatesData> | null => {
		if (!currentProduct) return null;
		return {
			path: { product_id: currentProduct.id },
			query: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
		};
	}, [currentProduct, pagination]);

	const { data, isLoading, error } = useQuery({
		...listEmailTemplatesOptions(queryOptions!),
		placeholderData: keepPreviousData,
		enabled: !!queryOptions,
	});

	function invalidateTemplates() {
		if (!currentProduct) return;

		void queryClient.invalidateQueries({
			predicate: (query) => {
				const key = query.queryKey[0] as
					| { _id?: string; path?: { product_id?: string } }
					| undefined;
				return (
					key?._id === "listEmailTemplates" &&
					key.path?.product_id === currentProduct.id
				);
			},
		});
	}

	const { mutate: createTemplate, isPending: isCreating } = useMutation({
		...createEmailTemplateMutation(),
		onSuccess: (tpl) => {
			invalidateTemplates();
			navigate({
				to: ROUTE_PATHS.EMAIL_TEMPLATE_BUILDER,
				params: { templateId: tpl.id },
			});
		},
		onError: (err: any) => {
			setCreateError(
				err?.errors?.[0]?.message ??
					err?.message ??
					"Failed to create template",
			);
		},
	});

	const { mutate: deleteTemplate } = useMutation({
		...deleteEmailTemplateMutation(),
		onSuccess: () => {
			invalidateTemplates();
		},
	});

	function handleNew() {
		if (!currentProduct) return;
		setCreateError(null);
		createTemplate({
			path: { product_id: currentProduct.id },
			body: {
				slug: uniqueSlug(),
				name: "New Template",
				subject: "Your subject here",
				body_html: "<p>Hello,</p><p>Edit this template to get started.</p>",
			},
		});
	}

	const columns = useMemo(
		() => [
			columnHelper.accessor("slug", {
				header: () => <span>Slug</span>,
				cell: (info) => (
					<span className="text-sm font-mono">{info.getValue()}</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("name", {
				header: () => <span>Name</span>,
				cell: (info) => (
					<Link
						to={ROUTE_PATHS.EMAIL_TEMPLATE_BUILDER}
						params={{ templateId: info.row.original.id }}
						className="text-sm font-medium hover:underline text-primary"
					>
						{info.getValue()}
					</Link>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("is_active", {
				header: () => <span>Status</span>,
				cell: (info) => (
					<span
						className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${info.getValue() ? "bg-green-100 text-green-800" : "bg-gray-100 text-gray-600"}`}
					>
						{info.getValue() ? "Active" : "Inactive"}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("published_version_id", {
				header: () => <span>Published</span>,
				cell: (info) => (
					<span
						className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${info.getValue() ? "bg-blue-100 text-blue-800" : "bg-yellow-100 text-yellow-800"}`}
					>
						{info.getValue() ? "Yes" : "Draft only"}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("created_at", {
				header: () => <span>Created</span>,
				cell: (info) => (
					<span className="text-sm text-muted-foreground">
						{dayjs(info.getValue()).format("D MMM YYYY")}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.display({
				id: "actions",
				header: () => <span>Actions</span>,
				cell: (info) => (
					<Button
						variant="destructive"
						size="sm"
						onClick={() => {
							if (!currentProduct) return;
							deleteTemplate({
								path: {
									product_id: currentProduct.id,
									email_template_id: info.row.original.id,
								},
							});
						}}
					>
						Delete
					</Button>
				),
			}),
		],
		[currentProduct, deleteTemplate],
	);

	if (!currentProduct) {
		return (
			<div className="flex items-center justify-center p-8">
				<p className="text-muted-foreground">
					Select a product to view email templates
				</p>
			</div>
		);
	}

	return (
		<div className="space-y-4">
			<div className="flex items-center justify-between">
				{createError && (
					<p className="text-sm text-destructive">{createError}</p>
				)}
				<div className="ml-auto">
					<Button size="sm" onClick={handleNew} disabled={isCreating}>
						{isCreating ? "Creating…" : "New Template"}
					</Button>
				</div>
			</div>
			<AnchorDataTable
				columns={columns}
				data={data?.items ?? []}
				loading={isLoading}
				total={data?.count ?? 0}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				enableRowSelection={false}
			/>
			{error && (
				<p className="text-sm text-destructive">
					Failed to load templates: {error.message}
				</p>
			)}
		</div>
	);
}
