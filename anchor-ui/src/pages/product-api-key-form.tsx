import { Page } from "@/components/common/Page";
import { ProductApiKeyForm } from "@/components/product/apikey/ProductApiKeyForm";
import { useProduct } from "@/hooks/useProduct";

interface ProductApiKeyFormPageProps {
	mode: "create" | "edit";
	apiKeyId?: string;
}

export default function ProductApiKeyFormPage({
	mode,
	apiKeyId,
}: ProductApiKeyFormPageProps) {
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
		<Page variant="wide" breadCrumbs={false}>
			<ProductApiKeyForm
				productId={currentProduct.id}
				productName={currentProduct.name}
				mode={mode}
				apiKeyId={apiKeyId}
			/>
		</Page>
	);
}
