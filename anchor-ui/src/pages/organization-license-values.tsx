import { getLicenseSchemaOptions } from "@/client/@tanstack/react-query.gen";
import { LicenseValueFields } from "@/components/license/LicenseValueFields";
import { useOrganizationLicenseQuery } from "@/components/license/use-organization-license";
import { useProduct } from "@/context/product/ProductContext";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";
import { useQuery } from "@tanstack/react-query";

export default function OrganizationLicenseValuesPage() {
	const { organizationId } = organizationLicenseDetailRoute.useParams();
	const { currentProduct } = useProduct();
	const productId = currentProduct?.id;

	const licenseQuery = useOrganizationLicenseQuery(productId, organizationId);
	const schemaQuery = useQuery({
		...getLicenseSchemaOptions({ path: { product_id: productId as string } }),
		enabled: !!productId,
		retry: false,
	});

	const license = licenseQuery.data;
	if (!license) return null;

	const schema = schemaQuery.data;

	return (
		<div className="flex flex-col gap-3">
			<p className="text-xs text-muted-foreground">
				Every field this product declares, and what this organization holds for
				it. Limits included.
			</p>
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
