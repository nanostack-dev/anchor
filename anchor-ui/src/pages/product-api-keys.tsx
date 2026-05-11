import { Page } from "@/components/common/Page";
import { ProductApiKeyDatatable } from "@/components/product/apikey/ProductApiKeyDatatable";
import { useProduct } from "@/hooks/useProduct";

export default function ProductAPIKeyPage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex items-center justify-center h-64">
					<p className="text-muted-foreground">
						Please select a product to manage API keys.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page>
			<div className="space-y-6">
				<div>
					<h1 className="text-3xl font-bold tracking-tight">API Keys</h1>
					<p className="text-muted-foreground">
						Manage API keys for {currentProduct.name}
					</p>
				</div>
				<ProductApiKeyDatatable productId={currentProduct.id} />
			</div>
		</Page>
	);
}
