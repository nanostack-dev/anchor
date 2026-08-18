import { Page } from "@/components/common/Page";
import { OrganizationLicenseDatatable } from "@/components/license/OrganizationLicenseDatatable";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { useProduct } from "@/context/product/ProductContext";
import { Building2 } from "lucide-react";

export default function OrganizationLicensePage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<Empty>
					<EmptyHeader>
						<EmptyMedia variant="icon">
							<Building2 />
						</EmptyMedia>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product to see which tier each of its organizations is on.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			</Page>
		);
	}

	return (
		<Page
			title="Organization Licenses"
			description="Which tier each organization is on. A license is instantiated and adjusted by the product backend; moving a set of organizations onto another tier is the one write this page makes."
		>
			<OrganizationLicenseDatatable productId={currentProduct.id} />
		</Page>
	);
}
