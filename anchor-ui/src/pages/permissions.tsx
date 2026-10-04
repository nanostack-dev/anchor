import { Page } from "@/components/common/Page";
import { ProductPermissionDatatable } from "@/components/product/permissions/ProductPermissionDatatable";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Inline, Text } from "@nanostackorg/design-system";

export default function PermissionsPage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<Inline space="xs" align="center">
					<Text tone="muted">
						Please select a product to manage permissions.
					</Text>
				</Inline>
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
