import {
	getLicenseSchemaOptions,
	getOrganizationLicenseOptions,
} from "@/client/@tanstack/react-query.gen";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { apiErrorHasCode, getErrorDetail } from "@/lib/api-error";
import { useQuery } from "@tanstack/react-query";
import dayjs from "dayjs";
import { BadgeCheck, Info, TriangleAlert } from "lucide-react";
import { LicenseValueFields } from "./LicenseValueFields";

const ORGANIZATION_LICENSE_NOT_FOUND = "ORGANIZATION_LICENSE_NOT_FOUND";

interface OrganizationLicensePanelProps {
	productId: string;
	organizationId: string;
}

/**
 * Read-only view of one organization's license. Organization licenses are
 * API-driven runtime data — instantiated and adjusted by the product backend,
 * never edited here — so this panel has no write path at all, unlike the
 * schema and template panels.
 */
export function OrganizationLicensePanel({
	productId,
	organizationId,
}: OrganizationLicensePanelProps) {
	const schemaQuery = useQuery({
		...getLicenseSchemaOptions({ path: { product_id: productId } }),
		retry: false,
	});

	const licenseQuery = useQuery({
		...getOrganizationLicenseOptions({
			path: { product_id: productId, organization_id: organizationId },
		}),
		retry: false,
	});

	if (schemaQuery.isLoading || licenseQuery.isLoading) {
		return (
			<div className="flex flex-col gap-2">
				<Skeleton className="h-9 w-full" />
				<Skeleton className="h-9 w-full" />
				<Skeleton className="h-9 w-full" />
			</div>
		);
	}

	const licenseNotFound = apiErrorHasCode(
		licenseQuery.error,
		ORGANIZATION_LICENSE_NOT_FOUND,
	);

	if (licenseQuery.error && !licenseNotFound) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon" className="text-destructive">
						<TriangleAlert />
					</EmptyMedia>
					<EmptyTitle>
						Couldn&rsquo;t load this organization&rsquo;s license
					</EmptyTitle>
					<EmptyDescription>
						{getErrorDetail(licenseQuery.error) ??
							"No response was received from the API. This app's requests never got an answer at all — a genuine offline/DNS/CORS failure, not a server-side error."}
					</EmptyDescription>
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

	if (licenseNotFound || !licenseQuery.data) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<BadgeCheck />
					</EmptyMedia>
					<EmptyTitle>No license</EmptyTitle>
					<EmptyDescription>
						This organization has not been instantiated onto a license template.
						Instantiation happens through the API — organization licenses are
						runtime data and not editable here.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	const license = licenseQuery.data;
	const schema = schemaQuery.data;

	return (
		<div className="flex flex-col gap-4">
			<dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
				<div className="rounded-lg bg-muted/50 p-4">
					<dt className="text-xs font-semibold text-muted-foreground">
						Template
					</dt>
					<dd className="mt-1 font-mono text-sm break-all">
						{license.template_id}
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

			<Alert>
				<Info />
				<AlertTitle>Usage and status are not shown yet</AlertTitle>
				<AlertDescription>
					Each limit&rsquo;s latest usage and derived status (
					<code>within_limit</code>, <code>at_limit</code>,{" "}
					<code>exceeded</code>, <code>stale</code>) depend on backend work that
					has not shipped yet. This view shows the values this
					organization&rsquo;s license currently holds.
				</AlertDescription>
			</Alert>

			{schema ? (
				<LicenseValueFields fields={schema.fields} values={license.values} />
			) : (
				<div className="divide-y divide-border rounded-lg border border-border">
					{Object.entries(license.values).map(([name, value]) => (
						<div key={name} className="flex items-center justify-between p-3">
							<span className="font-mono text-sm">{name}</span>
							<span className="text-sm">{String(value ?? "—")}</span>
						</div>
					))}
				</div>
			)}
		</div>
	);
}
