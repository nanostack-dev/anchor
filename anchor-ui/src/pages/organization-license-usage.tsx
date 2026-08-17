import { OrganizationLicenseLimits } from "@/components/license/OrganizationLicenseLimits";
import { UsageHistoryChart } from "@/components/license/UsageHistoryChart";
import { useOrganizationLicenseQuery } from "@/components/license/use-organization-license";
import { useProduct } from "@/context/product/ProductContext";
import {
	organizationLicenseDetailRoute,
	organizationLicenseUsageRoute,
} from "@/routes/organizations/organization-license.$organizationId";
import { useNavigate } from "@tanstack/react-router";

export default function OrganizationLicenseUsagePage() {
	const { organizationId } = organizationLicenseDetailRoute.useParams();
	const { field } = organizationLicenseUsageRoute.useSearch();
	const { currentProduct } = useProduct();
	const navigate = useNavigate();

	const licenseQuery = useOrganizationLicenseQuery(
		currentProduct?.id,
		organizationId,
	);
	const license = licenseQuery.data;

	if (!license || !currentProduct) return null;

	const usage = license.usage ?? {};
	const limitFields = Object.keys(usage).sort((a, b) => a.localeCompare(b));
	const chartedField = field && usage[field] ? field : limitFields[0];

	return (
		<div className="flex flex-col gap-6">
			<section className="flex flex-col gap-3">
				<p className="text-xs text-muted-foreground">
					Latest reported usage against what this organization is allowed.
					Anchor records usage past a limit and never blocks on it. Select a
					limit to see its history.
				</p>
				<OrganizationLicenseLimits
					usage={usage}
					selectedField={chartedField ?? null}
					onSelectField={(next) =>
						navigate({
							to: organizationLicenseUsageRoute.to,
							params: { organizationId },
							search: { field: next },
							replace: true,
						})
					}
				/>
			</section>

			{chartedField && (
				<UsageHistoryChart
					productId={currentProduct.id}
					organizationId={organizationId}
					field={chartedField}
					limit={usage[chartedField].limit}
				/>
			)}
		</div>
	);
}
