import { searchOrganizationLicensesOptions } from "@/client/@tanstack/react-query.gen";
import { listLicenseTemplatesOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { StatusBadge } from "@/components/common/StatusBadge";
import { OrganizationLicensePanel } from "@/components/license/OrganizationLicensePanel";
import { differsFromItsTemplate } from "@/components/license/license-migration-format";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { useProduct } from "@/context/product/ProductContext";
import { organizationLicenseRoute } from "@/routes/organizations/organization-license";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import { ArrowLeft, Building2 } from "lucide-react";

/**
 * One organization's license, on its own route rather than in a dialog: it is
 * the kind of thing an operator links a colleague to, reloads while a customer
 * is on the phone, and reads at full width.
 */
export default function OrganizationLicenseDetailPage() {
	const { organizationId } = organizationLicenseDetailRoute.useParams();
	const { currentProduct } = useProduct();
	const productId = currentProduct?.id;

	const summaryQuery = useQuery({
		...searchOrganizationLicensesOptions({
			path: { product_id: productId as string },
			body: {
				pagination: { limit: 1, offset: 0 },
				filter: { organization_ids: [organizationId] },
			},
		}),
		enabled: !!productId,
	});
	const summary = summaryQuery.data?.items?.[0];

	const templatesQuery = useQuery({
		...listLicenseTemplatesOptions({
			path: { product_id: productId as string },
		}),
		enabled: !!productId,
	});
	const template = summary?.license
		? templatesQuery.data?.items?.find(
				(item) => item.id === summary.license?.template_id,
			)
		: undefined;

	const backLink = (
		<Button
			variant="outline"
			render={<Link to={organizationLicenseRoute.to} />}
		>
			<ArrowLeft />
			All organizations
		</Button>
	);

	if (!currentProduct) {
		return (
			<Page breadCrumbs={false}>
				<Empty>
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<Building2 />
						</EmptyMedia>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product to read one of its organizations&rsquo; licenses.
						</EmptyDescription>
					</EmptyHeader>
					{backLink}
				</Empty>
			</Page>
		);
	}

	if (summaryQuery.isLoading) {
		return (
			<Page breadCrumbLabel="Loading" actions={backLink}>
				<div className="flex flex-col gap-3">
					<Skeleton className="h-9 w-64" />
					<Skeleton className="h-9 w-full" />
					<Skeleton className="h-9 w-full" />
				</div>
			</Page>
		);
	}

	if (!summary) {
		return (
			<Page breadCrumbs={false}>
				<Empty>
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<Building2 />
						</EmptyMedia>
						<EmptyTitle>No such organization</EmptyTitle>
						<EmptyDescription>
							This product has no organization with that identifier. It may have
							been deleted, or it belongs to another product.
						</EmptyDescription>
					</EmptyHeader>
					{backLink}
				</Empty>
			</Page>
		);
	}

	const differs =
		!!summary.license && differsFromItsTemplate(summary, template?.values);

	return (
		<Page
			breadCrumbLabel={summary.organization_name}
			title={summary.organization_name}
			description="What this organization is allowed, how much of each limit it has used, and every change ever made to its license."
			actions={backLink}
		>
			<div className="flex flex-col gap-6">
				<div className="flex flex-wrap items-center gap-2">
					{/* Only the exceptional states. The tier itself is named in the
						provenance below, next to the date it was stamped, and saying it
						twice within a hundred pixels is noise rather than emphasis. */}
					{!summary.license && (
						<StatusBadge tone="neutral">No license</StatusBadge>
					)}
					{template?.status === "ARCHIVED" && (
						<StatusBadge tone="warning">Tier withdrawn</StatusBadge>
					)}
					{differs && <StatusBadge tone="info">Adjusted</StatusBadge>}
				</div>

				<OrganizationLicensePanel
					productId={currentProduct.id}
					organizationId={organizationId}
				/>
			</div>
		</Page>
	);
}
