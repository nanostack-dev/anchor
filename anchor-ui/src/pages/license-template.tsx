import { getLicenseSchema, getLicenseTemplate } from "@/client";
import {
	getLicenseSchemaQueryKey,
	getLicenseTemplateQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import {
	LicenseTemplateBackLink,
	LicenseTemplateEditView,
} from "@/components/license/LicenseTemplateEditView";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { useProduct } from "@/context/product/ProductContext";
import { getErrorDetail } from "@/lib/api-error";
import { isHttpQueryError, unwrapQuery } from "@/lib/http-query-error";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

export default function LicenseTemplatePage({
	templateId,
	editing = false,
}: { templateId?: string; editing?: boolean }) {
	const { currentProduct } = useProduct();
	const productId = currentProduct?.id ?? "";
	const navigate = useNavigate();
	const schemaPath = { product_id: productId };
	const templatePath = { ...schemaPath, license_template_id: templateId ?? "" };
	const schemaQuery = useQuery({
		queryKey: getLicenseSchemaQueryKey({ path: schemaPath }),
		queryFn: ({ signal }) =>
			unwrapQuery(getLicenseSchema({ path: schemaPath, signal })),
		enabled: !!productId,
		retry: false,
	});
	const templateQuery = useQuery({
		queryKey: getLicenseTemplateQueryKey({ path: templatePath }),
		queryFn: ({ signal }) =>
			unwrapQuery(getLicenseTemplate({ path: templatePath, signal })),
		enabled: !!productId && !!templateId,
		retry: false,
	});
	const viewTemplate = (id: string) =>
		void navigate({
			to: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_DETAIL,
			params: { templateId: id },
			search: {},
			replace: true,
		});

	if (!currentProduct)
		return (
			<Page breadCrumbs={false}>
				<Empty>
					<EmptyHeader>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product to view its license templates.
						</EmptyDescription>
					</EmptyHeader>
					<LicenseTemplateBackLink />
				</Empty>
			</Page>
		);
	if (schemaQuery.isLoading || (templateId && templateQuery.isLoading))
		return (
			<Page breadCrumbs={false} title="Loading template">
				<div className="flex max-w-3xl flex-col gap-6">
					<div>
						<LicenseTemplateBackLink />
					</div>
					<Skeleton className="h-64 w-full" />
				</div>
			</Page>
		);
	const error = templateQuery.error ?? schemaQuery.error;
	if (error) {
		const notFound = isHttpQueryError(error) && error.status === 404;
		return (
			<Page breadCrumbs={false}>
				<Empty>
					<EmptyHeader>
						<EmptyTitle>
							{notFound
								? templateQuery.error
									? "Template not found"
									: "No license schema declared yet"
								: "Couldn’t load this template"}
						</EmptyTitle>
						<EmptyDescription>
							{notFound
								? "Check the selected product and return to its license templates."
								: (getErrorDetail(
										isHttpQueryError(error) ? error.body : error,
									) ?? "The request failed. Try again.")}
						</EmptyDescription>
					</EmptyHeader>
					<Button
						variant="outline"
						onClick={() => {
							void schemaQuery.refetch();
							if (templateId) void templateQuery.refetch();
						}}
					>
						Try again
					</Button>
					<LicenseTemplateBackLink />
				</Empty>
			</Page>
		);
	}
	if (!schemaQuery.data || (templateId && !templateQuery.data)) return null;
	return (
		<LicenseTemplateEditView
			key={`${productId}:${templateId ?? "new"}`}
			productId={productId}
			schema={schemaQuery.data}
			template={templateId ? templateQuery.data : undefined}
			editing={editing}
			onEdit={() => {
				if (templateId)
					void navigate({
						to: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_DETAIL,
						params: { templateId },
						search: { edit: true },
					});
			}}
			onCancel={() =>
				templateId
					? viewTemplate(templateId)
					: void navigate({ to: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATES })
			}
			onSaved={(template) => viewTemplate(template.id)}
		/>
	);
}
