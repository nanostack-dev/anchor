import {
	listLicenseTemplatesOptions,
	searchOrganizationLicensesOptions,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { StatusBadge } from "@/components/common/StatusBadge";
import { OrganizationLicenseIdentity } from "@/components/license/OrganizationLicenseIdentity";
import { OrganizationLicenseTabs } from "@/components/license/OrganizationLicenseTabs";
import { useOrganizationLicenseQuery } from "@/components/license/use-organization-license";
import { useProduct } from "@/context/product/ProductContext";
import { getErrorDetail } from "@/lib/api-error";
import { isHttpQueryError } from "@/lib/http-query-error";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";

import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	ArrowLeftIcon as ArrowLeft,
	SealCheckIcon as BadgeCheck,
	BuildingsIcon as Building2,
	WarningIcon as TriangleAlert,
} from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
import { Outlet } from "@tanstack/react-router";

/**
 * One organization's license, on its own route rather than in a dialog: it is
 * the kind of thing an operator links a colleague to, reloads while a customer
 * is on the phone, and reads at full width.
 *
 * Identity and provenance stay here, above the tabs. Everything a tab shows is
 * a different question about the same license, and none of them answers "which
 * customer am I looking at, and on which tier".
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

	const licenseQuery = useOrganizationLicenseQuery(productId, organizationId);

	const backLink = (
		<ButtonLink
			href={ROUTE_PATHS.ORGANIZATION_LICENSE}
			icon={ArrowLeft}
			variant="outline"
		>
			All organizations
		</ButtonLink>
	);

	if (!currentProduct) {
		return (
			<Page breadCrumbs={false}>
				<Empty>
					<EmptyHeader>
						<EmptyMedia icon={Building2} />
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

	if (summaryQuery.isLoading || licenseQuery.isLoading) {
		return (
			<Page
				breadCrumbLabels={{ [organizationId]: "Loading" }}
				actions={backLink}
			>
				<Stack space="md">
					<Skeleton height="lg" width="1/2" />
					<Skeleton height="lg" />
					<Skeleton height="lg" />
				</Stack>
			</Page>
		);
	}

	// An unanswered lookup is not an absent customer. Reporting an outage as a
	// deletion sends an operator to look for a record that is still there.
	if (summaryQuery.error) {
		return (
			<Page breadCrumbs={false}>
				<Empty>
					<EmptyHeader>
						<EmptyMedia icon={TriangleAlert} />
						<EmptyTitle>Couldn&rsquo;t load this organization</EmptyTitle>
						<EmptyDescription>
							{getErrorDetail(summaryQuery.error) ??
								"The request for this organization did not come back."}
						</EmptyDescription>
					</EmptyHeader>
					<Button
						variant="outline"
						size="sm"
						onClick={() => void summaryQuery.refetch()}
					>
						Try again
					</Button>
					{backLink}
				</Empty>
			</Page>
		);
	}

	if (!summary) {
		return (
			<Page breadCrumbs={false}>
				<Empty>
					<EmptyHeader>
						<EmptyMedia icon={Building2} />
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

	// This route documents exactly one 404 case — the organization has no
	// license yet — so any 404 here is treated as that, whether or not its body
	// happened to parse into the specific ORGANIZATION_LICENSE_NOT_FOUND shape.
	const licenseNotFound =
		isHttpQueryError(licenseQuery.error) && licenseQuery.error.status === 404;

	const licenseBody = () => {
		if (licenseQuery.error && !licenseNotFound) {
			const error = licenseQuery.error;
			const detail = isHttpQueryError(error)
				? (getErrorDetail(error.body) ??
					`The server responded with HTTP ${error.status}.`)
				: (getErrorDetail(error) ??
					"No response was received at all — the request never reached a server, or a browser-level failure (offline, DNS, CORS) stopped it before one could answer.");

			return (
				<Empty>
					<EmptyHeader>
						<EmptyMedia icon={TriangleAlert} />
						<EmptyTitle>
							Couldn&rsquo;t load this organization&rsquo;s license
						</EmptyTitle>
						<EmptyDescription>{detail}</EmptyDescription>
					</EmptyHeader>
					<Button
						variant="outline"
						size="sm"
						onClick={() => void licenseQuery.refetch()}
					>
						Try again
					</Button>
				</Empty>
			);
		}

		const license = licenseQuery.data;
		if (licenseNotFound || !license) {
			return (
				<Empty>
					<EmptyHeader>
						<EmptyMedia icon={BadgeCheck} />
						<EmptyTitle>No license</EmptyTitle>
						<EmptyDescription>
							This organization has not been instantiated onto a license
							template. Instantiation happens through the API — organization
							licenses are runtime data and not editable here.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			);
		}

		return (
			<>
				<OrganizationLicenseIdentity
					templateName={
						templatesQuery.data?.items?.find(
							(item) => item.id === license.template_id,
						)?.name ?? license.template_id
					}
					instantiatedAt={license.instantiated_at}
				/>

				<Stack space="lg">
					<OrganizationLicenseTabs organizationId={organizationId} />
					<Outlet />
				</Stack>
			</>
		);
	};

	return (
		<Page
			breadCrumbLabels={{ [organizationId]: summary.organization_name }}
			title={summary.organization_name}
			description="What this organization is allowed, how much of each limit it has used, and every change ever made to its license."
			actions={backLink}
		>
			<Stack space="xl">
				{(!summary.license || template?.status === "ARCHIVED") && (
					<Inline space="sm">
						{!summary.license && (
							<StatusBadge tone="neutral">No license</StatusBadge>
						)}
						{template?.status === "ARCHIVED" && (
							<StatusBadge tone="warning">Tier withdrawn</StatusBadge>
						)}
					</Inline>
				)}
				{licenseBody()}
			</Stack>
		</Page>
	);
}
