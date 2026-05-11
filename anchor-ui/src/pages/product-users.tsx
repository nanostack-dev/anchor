import { Page } from "@/components/common/Page";
import { ProductUserDatatable } from "@/components/product/user/ProductUserDatatable";

export default function ProductUsersPage() {
	return (
		<Page
			title={"Users"}
			description={"View and manage users within the selected product."}
		>
			<ProductUserDatatable />
		</Page>
	);
}
