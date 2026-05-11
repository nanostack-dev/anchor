import { Page } from "@/components/common/Page";
import { ProductDatatable } from "@/components/product/ProductDatatable";

export default function ProductsPage() {
	return (
		<Page
			title="Products"
			description="Manage and review products. Search, filter, and take actions."
			variant="full"
		>
			<ProductDatatable />
		</Page>
	);
}
