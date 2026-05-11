import { Page } from "@/components/common/Page";
import { ProductRoleDatatable } from "@/components/product/roles/ProductRoleDatatable";
import { useProduct } from "@/hooks/useProduct";

export default function ProductRolesPage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex items-center justify-center h-64">
					<p className="text-muted-foreground">
						Please select a product to manage roles.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page>
			<div className="space-y-6">
				<div>
					<h1 className="text-3xl font-bold tracking-tight">Roles</h1>
					<p className="text-muted-foreground">
						Manage roles for {currentProduct.name}
					</p>
				</div>
				<ProductRoleDatatable productId={currentProduct.id} />
			</div>
		</Page>
	);
}
