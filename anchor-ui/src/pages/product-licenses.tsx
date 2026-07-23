import { Page } from "@/components/common/Page";
import { LicenseDatatable } from "@/components/product/licensing/LicenseDatatable";
import { useProduct } from "@/hooks/useProduct";

export default function ProductLicensesPage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex items-center justify-center h-64">
					<p className="text-muted-foreground">
						Please select a product to manage licenses.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page
			title="Licenses"
			description={`Assign plans to ${currentProduct.name} organizations and manage license lifecycle, expiry and per-organization overrides.`}
			variant="full"
		>
			<LicenseDatatable productId={currentProduct.id} />
		</Page>
	);
}
