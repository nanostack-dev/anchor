import { getProductOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { ProductEventsForm } from "@/components/product/ProductEventsForm";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@/components/ui/empty";
import { useProduct } from "@/hooks/useProduct";
import { useQuery } from "@tanstack/react-query";

export default function ProductEventsPage() {
	const { currentProduct, refreshProducts } = useProduct();
	const productQuery = useQuery({
		...getProductOptions({
			path: { product_id: currentProduct?.id ?? "" },
		}),
		enabled: Boolean(currentProduct),
		refetchInterval: 30_000,
	});

	return (
		<Page
			title="Events"
			description="Outbound webhook endpoint for this product. Anchor POSTs signed catalog events to the URL you save."
			variant="full"
		>
			{currentProduct ? (
				<ProductEventsForm
					key={currentProduct.id}
					product={productQuery.data ?? currentProduct}
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
