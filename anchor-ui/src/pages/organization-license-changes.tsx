import { LicenseSectionIntro } from "@/components/license/LicenseSectionIntro";
import { OrganizationLicenseHistory } from "@/components/license/OrganizationLicenseHistory";
import { useProduct } from "@/context/product/ProductContext";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";
import { Stack } from "@nanostackorg/design-system/layout/stack";

export default function OrganizationLicenseChangesPage() {
	const { organizationId } = organizationLicenseDetailRoute.useParams();
	const { currentProduct } = useProduct();

	if (!currentProduct) return null;

	return (
		<Stack space="md">
			<LicenseSectionIntro>
				What this organization was given, and each later adjustment. Newest
				first. Entries are not edited.
			</LicenseSectionIntro>
			<OrganizationLicenseHistory
				productId={currentProduct.id}
				organizationId={organizationId}
			/>
		</Stack>
	);
}
