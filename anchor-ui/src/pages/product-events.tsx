import { Page } from "@/components/common/Page";
import { ProductEventsForm } from "@/components/product/ProductEventsForm";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@/components/ui/empty";
import { useProduct } from "@/hooks/useProduct";

export default function ProductEventsPage() {
	const { currentProduct, refreshProducts } = useProduct();

	return (
		<Page
			title="Events"
			description="Outbound webhook endpoint for this product. Anchor POSTs signed catalog events to the URL you save."
			variant="full"
		>
			{currentProduct ? (
				<ProductEventsForm
					product={currentProduct}
					productId={currentProduct.id}
					onSaved={refreshProducts}
				/>
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product from the top bar to configure its event endpoint.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
