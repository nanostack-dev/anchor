import { OrganizationLicenseHistory } from "@/components/license/OrganizationLicenseHistory";
import { useProduct } from "@/context/product/ProductContext";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";

export default function OrganizationLicenseChangesPage() {
	const { organizationId } = organizationLicenseDetailRoute.useParams();
	const { currentProduct } = useProduct();

	if (!currentProduct) return null;

	return (
		<div className="flex flex-col gap-3">
			<p className="text-xs text-muted-foreground">
				What this organization was given, and each later adjustment. Newest
				first. Entries are not edited.
			</p>
			<OrganizationLicenseHistory
				productId={currentProduct.id}
				organizationId={organizationId}
			/>
		</div>
	);
}
