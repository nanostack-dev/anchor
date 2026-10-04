import { Page } from "@/components/common/Page";
import { ProductApiKeyForm } from "@/components/product/apikey/ProductApiKeyForm";
import { useProduct } from "@/hooks/useProduct";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";

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
				<EmptyState
					title="No product selected"
					description="Please select a product to manage API keys."
				/>
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
