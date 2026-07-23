import { Page } from "@/components/common/Page";
import { PlanDatatable } from "@/components/product/licensing/PlanDatatable";
import { useProduct } from "@/hooks/useProduct";

export default function ProductPlansPage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex items-center justify-center h-64">
					<p className="text-muted-foreground">
						Please select a product to manage plans.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page
			title="Plans"
			description={`Define the plans of ${currentProduct.name} and the entitlements each plan grants.`}
			variant="full"
		>
			<PlanDatatable productId={currentProduct.id} />
		</Page>
	);
}
