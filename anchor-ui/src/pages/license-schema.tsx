import { Page } from "@/components/common/Page";
import { LicenseSchemaPanel } from "@/components/license/LicenseSchemaPanel";
import { useProduct } from "@/context/product/ProductContext";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";

export default function LicenseSchemaPage() {
	const { currentProduct } = useProduct();

	return (
		<Page
			title="License Schema"
			description="Every field a license may carry for this product: its type and validation rules."
		>
			{currentProduct ? (
				<LicenseSchemaPanel productId={currentProduct.id} />
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product from the top bar to view its license schema.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
