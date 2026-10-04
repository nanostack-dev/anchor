import { Page } from "@/components/common/Page";
import { ProductApiKeyDatatable } from "@/components/product/apikey/ProductApiKeyDatatable";
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
			title="API Keys"
			description={
				currentProduct
					? `Manage api keys for ${currentProduct.name}`
					: undefined
			}
		>
			{currentProduct ? (
				<ProductApiKeyDatatable productId={currentProduct.id} />
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>Select a product</EmptyTitle>
						<EmptyDescription>
							Please select a product to manage api keys.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
