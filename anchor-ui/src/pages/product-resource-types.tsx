import { Page } from "@/components/common/Page";
import { ResourceTypesEditor } from "@/components/product/resource-types/ResourceTypesEditor";
import { useProduct } from "@/hooks/useProduct";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";

export default function ProductResourceTypesPage() {
	const { currentProduct } = useProduct();
	return (
		<Page
			title="Resource types"
			description={
				currentProduct
					? `The kinds of things ${currentProduct.name} protects, from the organization down. Roles and permissions are scoped to one of them.`
					: undefined
			}
		>
			{currentProduct ? (
				<ResourceTypesEditor
					key={currentProduct.id}
					productId={currentProduct.id}
				/>
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>Select a product</EmptyTitle>
						<EmptyDescription>
							Please select a product to manage its resource types.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
