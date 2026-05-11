import { Page } from "@/components/common/Page";
import { OrganizationDatatable } from "@/components/organization/OrganizationDatatable";

export default function OrganizationsPage() {
	return (
		<Page
			title={"Product Organizations"}
			description={"View and manage organizations within the selected product."}
		>
			<OrganizationDatatable />
		</Page>
	);
}
