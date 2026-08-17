import {
	listLicenseTemplatesOptions,
	searchOrganizationLicensesOptions,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { StatusBadge } from "@/components/common/StatusBadge";
import { OrganizationLicenseTabs } from "@/components/license/OrganizationLicenseTabs";
import { differsFromItsTemplate } from "@/components/license/license-migration-format";
import { useOrganizationLicenseQuery } from "@/components/license/use-organization-license";
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
import { getErrorDetail } from "@/lib/api-error";
import { isHttpQueryError } from "@/lib/http-query-error";
import { organizationLicenseRoute } from "@/routes/organizations/organization-license";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";
import { useQuery } from "@tanstack/react-query";
import { Link, Outlet } from "@tanstack/react-router";
import dayjs from "dayjs";
import { ArrowLeft, BadgeCheck, Building2, TriangleAlert } from "lucide-react";

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
		<Button
			variant="outline"
			nativeButton={false}
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

	if (summaryQuery.isLoading || licenseQuery.isLoading) {
		return (
			<Page
				breadCrumbLabels={{ [organizationId]: "Loading" }}
				actions={backLink}
			>
				<div className="flex flex-col gap-3">
					<Skeleton className="h-9 w-64" />
					<Skeleton className="h-9 w-full" />
					<Skeleton className="h-9 w-full" />
				</div>
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
						<EmptyMedia variant="icon" className="text-destructive">
							<TriangleAlert />
						</EmptyMedia>
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
		!!summary.license &&
		differsFromItsTemplate(summary, template?.values) === true;

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
						<EmptyMedia variant="icon" className="text-destructive">
							<TriangleAlert />
						</EmptyMedia>
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
						<EmptyMedia variant="icon">
							<BadgeCheck />
						</EmptyMedia>
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
				<dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
					<div className="rounded-lg bg-muted/50 p-4">
						<dt className="text-xs font-semibold text-muted-foreground">
							Template
						</dt>
						<dd className="mt-1 text-sm font-medium">
							{templatesQuery.data?.items?.find(
								(item) => item.id === license.template_id,
							)?.name ?? license.template_id}
						</dd>
					</div>
					<div className="rounded-lg bg-muted/50 p-4">
						<dt className="text-xs font-semibold text-muted-foreground">
							Instantiated
						</dt>
						<dd className="mt-1 text-sm">
							{dayjs(license.instantiated_at).format("D MMMM YYYY H:mm")}
						</dd>
					</div>
				</dl>

				<div className="flex flex-col gap-4">
					<OrganizationLicenseTabs organizationId={organizationId} />
					<Outlet />
				</div>
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
				{licenseBody()}
			</div>
		</Page>
	);
}
