import { getOrganizationLicense } from "@/client";
import {
	getLicenseSchemaOptions,
	getOrganizationLicenseQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { getErrorDetail } from "@/lib/api-error";
import { isHttpQueryError, unwrapQuery } from "@/lib/http-query-error";
import { useQuery } from "@tanstack/react-query";
import dayjs from "dayjs";
import { BadgeCheck, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { LicenseValueFields } from "./LicenseValueFields";
import { OrganizationLicenseLimits } from "./OrganizationLicenseLimits";
import { UsageHistoryChart } from "./UsageHistoryChart";

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
	const [selectedField, setSelectedField] = useState<string | null>(null);

	const schemaQuery = useQuery({
		...getLicenseSchemaOptions({ path: { product_id: productId } }),
		retry: false,
	});

	const licenseQuery = useQuery({
		queryKey: getOrganizationLicenseQueryKey({
			path: { product_id: productId, organization_id: organizationId },
		}),
		// Raw (non-throwing) SDK call, not getOrganizationLicenseOptions(): see
		// the matching comment in LicenseSchemaPanel — the *Options() helper's
		// throwOnError mode loses the HTTP status once a response body doesn't
		// parse as an ApiErrorResponse, which an empty or plain-text 404 body
		// from an earlier middleware layer will do.
		queryFn: () =>
			unwrapQuery(
				getOrganizationLicense({
					path: { product_id: productId, organization_id: organizationId },
				}),
			),
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

	// This route documents exactly one 404 case — the organization has no
	// license yet — so any 404 here is treated as that, whether or not its
	// body happened to parse into the specific ORGANIZATION_LICENSE_NOT_FOUND
	// shape.
	const licenseNotFound =
		isHttpQueryError(licenseQuery.error) && licenseQuery.error.status === 404;

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
	const usage = license.usage ?? {};
	const limitFields = Object.keys(usage).sort((a, b) => a.localeCompare(b));
	const chartedField =
		selectedField && usage[selectedField] ? selectedField : limitFields[0];

	return (
		<div className="flex flex-col gap-6">
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

			<section className="flex flex-col gap-3">
				<div className="flex flex-col gap-0.5">
					<h2 className="text-sm font-semibold">Limits</h2>
					<p className="text-xs text-muted-foreground">
						Latest reported usage against what this organization is allowed.
						Anchor records usage past a limit and never blocks on it. Select a
						limit to see its history.
					</p>
				</div>
				<OrganizationLicenseLimits
					usage={usage}
					selectedField={chartedField ?? null}
					onSelectField={setSelectedField}
				/>
			</section>

			{chartedField && (
				<UsageHistoryChart
					productId={productId}
					organizationId={organizationId}
					field={chartedField}
					limit={usage[chartedField].limit}
				/>
			)}

			<section className="flex flex-col gap-3">
				<h2 className="text-sm font-semibold">All license values</h2>
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
			</section>
		</div>
	);
}
