import { Page } from "@/components/common/Page";
import { ProductResourcePermissionDatatable } from "@/components/product/permissions/ProductResourcePermissionDatatable";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";

export default function ProductResourcePermissionsPage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex items-center justify-center h-64">
					<p className="text-muted-foreground">
						Please select a product to manage permissions.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page
			title="Resources Permissions"
			description={`Manage resource permissions for ${currentProduct.name}`}
			pageInfo={{
				title: "Product Resources Permissions",
				description:
					'These are custom permissions that you define for your product (e.g., "file:read", "user:write"). They are set by the admin of anchor and represent specific actions within your product\'s domain.',
				linkTo: ROUTE_PATHS.PRODUCT_PERMISSIONS,
			}}
		>
			<ProductResourcePermissionDatatable productId={currentProduct.id} />
		</Page>
	);
}
