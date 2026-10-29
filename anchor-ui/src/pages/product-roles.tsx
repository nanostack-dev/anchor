import { Page } from "@/components/common/Page";
import { ProductRoleDatatable } from "@/components/product/roles/ProductRoleDatatable";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { useNavigate } from "@tanstack/react-router";

interface ProductRolesPageProps {
	resourceTypes?: string[];
}

export default function ProductRolesPage({
	resourceTypes = [],
}: ProductRolesPageProps) {
	const { currentProduct } = useProduct();
	const navigate = useNavigate();
	return (
		<Page
			title="Roles"
			description={
				currentProduct
					? `Role templates of ${currentProduct.name}. Every organization gets its own copy of each, and its members hold those copies.`
					: undefined
			}
		>
			{currentProduct ? (
				<ProductRoleDatatable
					key={currentProduct.id}
					productId={currentProduct.id}
					resourceTypes={resourceTypes}
					onResourceTypesChange={(slugs) =>
						void navigate({
							to: ROUTE_PATHS.PRODUCT_ROLES,
							search: {
								resource_type: slugs.length ? slugs.join(",") : undefined,
							},
							replace: true,
						})
					}
				/>
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>Select a product</EmptyTitle>
						<EmptyDescription>
							Please select a product to manage roles.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
