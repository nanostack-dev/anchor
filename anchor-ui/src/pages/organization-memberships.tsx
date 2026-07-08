import { searchProductOrganizationsOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { OrganizationMembershipDatatable } from "@/components/organization/OrganizationMembershipDatatable";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { useProduct } from "@/context/product/ProductContext";
import { useQuery } from "@tanstack/react-query";
import { Building2 } from "lucide-react";
import { useState } from "react";

export default function OrganizationMembershipsPage() {
	const { currentProduct } = useProduct();
	const [selectedOrgId, setSelectedOrgId] = useState<string>("");

	const { data: orgsData, isLoading } = useQuery({
		...searchProductOrganizationsOptions({
			path: { product_id: currentProduct?.id as string },
			body: { pagination: { limit: 100, offset: 0 } },
		}),
		enabled: !!currentProduct?.id,
	});

	const organizations = orgsData?.items || [];

	return (
		<Page
			title="Organization Members"
			description="View and manage members across your product's organizations."
		>
			<div className="flex flex-col gap-6">
				<div className="flex w-full max-w-sm flex-col gap-2">
					<label
						htmlFor="org-select"
						className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70"
					>
						Organization
					</label>
					<Select
						value={selectedOrgId || undefined}
						onValueChange={(value) => setSelectedOrgId(value ?? "")}
						disabled={isLoading || organizations.length === 0}
					>
						<SelectTrigger id="org-select">
							<SelectValue
								placeholder={
									isLoading
										? "Loading organizations..."
										: organizations.length === 0
											? "No organizations found"
											: "Select an organization..."
								}
							/>
						</SelectTrigger>
						<SelectContent>
							{organizations.map((org) => (
								<SelectItem key={org.id} value={org.id}>
									<div className="flex items-center gap-2">
										<Building2 className="size-4" />
										<span>{org.name}</span>
									</div>
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>

				{selectedOrgId ? (
					<OrganizationMembershipDatatable organizationId={selectedOrgId} />
				) : (
					<div className="flex flex-col items-center justify-center p-12 text-center border border-border rounded-lg bg-muted">
						<Building2 className="size-10 text-muted-foreground mb-4" />
						<h3 className="text-lg font-medium">No Organization Selected</h3>
						<p className="text-sm text-muted-foreground mt-1 max-w-sm">
							Please select an organization from the dropdown above to view its
							members.
						</p>
					</div>
				)}
			</div>
		</Page>
	);
}
