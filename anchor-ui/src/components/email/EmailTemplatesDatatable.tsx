import type {
	ApiErrorResponse,
	EmailTemplateResponse,
	ListEmailTemplatesData,
	Options,
} from "@/client";
import {
	createEmailTemplateMutation,
	deleteEmailTemplateMutation,
	listEmailTemplatesOptions,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import { Button } from "@/components/ui/button";
import { useProduct } from "@/context/product/ProductContext";
import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	keepPreviousData,
	useMutation,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import dayjs from "dayjs";
import { useMemo, useState } from "react";

const columnHelper = createColumnHelper<EmailTemplateResponse>();

function getApiErrorMessage(error: unknown, fallback: string): string {
	if (
		typeof error === "object" &&
		error !== null &&
		"errors" in error &&
		Array.isArray((error as ApiErrorResponse).errors)
	) {
		return (error as ApiErrorResponse).errors[0]?.message ?? fallback;
	}

	if (error instanceof Error && error.message) {
		return error.message;
	}

	return fallback;
}

function uniqueSlug() {
	return `template-${Date.now().toString(36)}`;
}

export function EmailTemplatesDatatable() {
	const { currentProduct } = useProduct();
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	const productId = currentProduct?.id ?? "";

	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 20,
	});
	const [sorting, setSorting] = useState<SortingState>([]);
	const [createError, setCreateError] = useState<string | null>(null);

	const queryOptions = useMemo((): Options<ListEmailTemplatesData> => {
		return {
			path: { product_id: productId },
			query: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
		};
	}, [pagination, productId]);

	const { data, isLoading, error, refetch } = useQuery({
		...listEmailTemplatesOptions(queryOptions),
		placeholderData: keepPreviousData,
		enabled: !!currentProduct,
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
		onError: (err) => {
			setCreateError(getApiErrorMessage(err, "Failed to create template"));
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
					<StatusBadge tone={info.getValue() ? "success" : "neutral"}>
						{info.getValue() ? "Active" : "Inactive"}
					</StatusBadge>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("published_version_id", {
				header: () => <span>Published</span>,
				cell: (info) => (
					<StatusBadge tone={info.getValue() ? "info" : "warning"}>
						{info.getValue() ? "Published" : "Draft"}
					</StatusBadge>
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
		<div className="flex flex-col gap-4">
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
				resourceName="templates"
				error={error}
				onRetry={() => {
					void refetch();
				}}
				total={data?.count ?? 0}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				enableRowSelection={false}
			/>
		</div>
	);
}
