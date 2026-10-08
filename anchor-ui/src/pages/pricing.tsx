import { Page } from "@/components/common/Page";
import { useProduct } from "@/context/product/ProductContext";
import { createBillingAPI } from "@/features/billing/billing-api";
import { BillingWorkspace } from "@/features/billing/billing-workspace";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system";
import { useMemo } from "react";

export default function PricingPage() {
	const { currentProduct } = useProduct();
	const productId = currentProduct?.id;
	const api = useMemo(
		() => (productId ? createBillingAPI(productId) : null),
		[productId],
	);
	return (
		<Page
			title="Pricing"
			description="Connect recurring Stripe prices to this product’s license templates."
		>
			{currentProduct && api ? (
				<BillingWorkspace
					key={currentProduct.id}
					api={api}
					productId={currentProduct.id}
				/>
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product from the top bar to manage its pricing.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
