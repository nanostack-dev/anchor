import { Page } from "@/components/common/Page";
import { LicenseTemplateDatatable } from "@/components/license/LicenseTemplateDatatable";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@/components/ui/empty";
import { useProduct } from "@/context/product/ProductContext";

export default function LicenseTemplatesPage() {
	const { currentProduct } = useProduct();

	return (
		<Page
			title="License Templates"
			description="Named, reusable sets of values satisfying this product's license schema — the tiers an organization can be instantiated onto."
		>
			{currentProduct ? (
				<LicenseTemplateDatatable productId={currentProduct.id} />
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product from the top bar to view its license templates.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
