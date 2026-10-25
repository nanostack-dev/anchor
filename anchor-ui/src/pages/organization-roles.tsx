import { OrganizationRolesTable } from "@/components/organization/OrganizationRolesTable";
import { useProduct } from "@/context/product/ProductContext";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";

export default function OrganizationRolesPage() {
	const { organizationId } = organizationLicenseDetailRoute.useParams();
	const { currentProduct } = useProduct();
	if (!currentProduct) return null;
	return (
		<OrganizationRolesTable
			key={`${currentProduct.id}:${organizationId}`}
			productId={currentProduct.id}
			organizationId={organizationId}
		/>
	);
}
