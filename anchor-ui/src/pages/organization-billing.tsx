import { getOrganizationLicenseQueryKey } from "@/client/@tanstack/react-query.gen";
import { useProduct } from "@/context/product/ProductContext";
import { createBillingAPI } from "@/features/billing/billing-api";
import { BillingWorkspace } from "@/features/billing/billing-workspace";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";
import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";

export default function OrganizationBillingPage() {
	const { organizationId } = organizationLicenseDetailRoute.useParams();
	const { currentProduct } = useProduct();
	const queryClient = useQueryClient();
	const productId = currentProduct?.id ?? "";
	const api = useMemo(() => createBillingAPI(productId), [productId]);
	const invalidateLicense = useCallback(() => {
		void queryClient.invalidateQueries({
			queryKey: getOrganizationLicenseQueryKey({
				path: { product_id: productId, organization_id: organizationId },
			}),
		});
		void queryClient.invalidateQueries({
			predicate: (query) => {
				const key = query.queryKey[0];
				return (
					typeof key === "object" &&
					key !== null &&
					"_id" in key &&
					key._id === "searchOrganizationLicenses" &&
					"path" in key &&
					typeof key.path === "object" &&
					key.path !== null &&
					"product_id" in key.path &&
					key.path.product_id === productId
				);
			},
		});
	}, [queryClient, productId, organizationId]);
	return productId ? (
		<BillingWorkspace
			key={`${productId}:${organizationId}`}
			api={api}
			productId={productId}
			organizationId={organizationId}
			onLicenseChanged={invalidateLicense}
		/>
	) : null;
}
