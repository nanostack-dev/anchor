import { Page } from "@/components/common/Page";
import { ProductRoleDatatable } from "@/components/product/roles/ProductRoleDatatable";
import { useProduct } from "@/hooks/useProduct";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";

export default function ResourcePage() {
	const { currentProduct } = useProduct();
	return (
		<Page
			title="Roles"
			description={
				currentProduct ? `Manage roles for ${currentProduct.name}` : undefined
			}
		>
			{currentProduct ? (
				<ProductRoleDatatable productId={currentProduct.id} />
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
