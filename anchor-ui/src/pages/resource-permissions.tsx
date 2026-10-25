import { Page } from "@/components/common/Page";
import { ProductResourcePermissionDatatable } from "@/components/product/permissions/ProductResourcePermissionDatatable";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Inline, Text } from "@nanostackorg/design-system";
import { useNavigate } from "@tanstack/react-router";

interface ProductResourcePermissionsPageProps {
	resourceTypes?: string[];
}

export default function ProductResourcePermissionsPage({
	resourceTypes = [],
}: ProductResourcePermissionsPageProps) {
	const { currentProduct } = useProduct();
	const navigate = useNavigate();

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
			title="Resources Permissions"
			description={`Manage resource permissions for ${currentProduct.name}. Each one is scoped to a resource type.`}
			pageInfo={{
				title: "Product Resources Permissions",
				description:
					'These are custom permissions that you define for your product (e.g., "file:read", "user:write"). They are set by the admin of anchor and represent specific actions within your product\'s domain.',
				linkTo: ROUTE_PATHS.PRODUCT_PERMISSIONS,
			}}
		>
			<ProductResourcePermissionDatatable
				key={currentProduct.id}
				productId={currentProduct.id}
				resourceTypes={resourceTypes}
				onResourceTypesChange={(slugs) =>
					void navigate({
						to: ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS,
						search: {
							resource_type: slugs.length ? slugs.join(",") : undefined,
						},
						replace: true,
					})
				}
			/>
		</Page>
	);
}
