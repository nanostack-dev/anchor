import { Page } from "@/components/common/Page";
import { ProductPermissionDatatable } from "@/components/product/permissions/ProductPermissionDatatable";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";

export default function PermissionsPage() {
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
			title="Permissions"
			description="Available Anchor Permissions to manage your product"
			pageInfo={{
				title: "Product Permissions",
				description:
					"These are system-level permissions that manage anchor itself and are managed by the software. They can only be assigned to Product API Keys and control access to core anchor functionality.",
				linkTo: ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS,
			}}
		>
			<ProductPermissionDatatable productId={currentProduct.id} />
		</Page>
	);
}
