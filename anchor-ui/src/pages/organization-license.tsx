import { searchProductOrganizationsOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { OrganizationLicensePanel } from "@/components/license/OrganizationLicensePanel";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { useProduct } from "@/context/product/ProductContext";
import { useQuery } from "@tanstack/react-query";
import { BadgeCheck, Building2 } from "lucide-react";
import { useState } from "react";

export default function OrganizationLicensePage() {
	const { currentProduct } = useProduct();
	const [selectedOrgId, setSelectedOrgId] = useState("");

	const { data: organizationsData, isLoading } = useQuery({
		...searchProductOrganizationsOptions({
			path: { product_id: currentProduct?.id as string },
			body: { pagination: { limit: 100, offset: 0 } },
		}),
		enabled: !!currentProduct?.id,
	});

	const organizations = organizationsData?.items ?? [];

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex h-64 items-center justify-center">
					<p className="text-muted-foreground">
						Please select a product to view an organization&rsquo;s license.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page
			title="Organization License"
			description="Read-only: an organization's license is API-driven runtime data, instantiated and adjusted by the product backend."
		>
			<div className="flex flex-col gap-6">
				<div className="flex w-full max-w-sm flex-col gap-2">
					<label
						htmlFor="organization-license-org-select"
						className="text-sm font-medium leading-none"
					>
						Organization
					</label>
					<Select
						items={organizations.map((organization) => ({
							value: organization.id,
							label: organization.name,
						}))}
						value={selectedOrgId ?? null}
						onValueChange={(value) => setSelectedOrgId(value ?? "")}
						disabled={isLoading || organizations.length === 0}
					>
						<SelectTrigger id="organization-license-org-select">
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
							{organizations.map((organization) => (
								<SelectItem key={organization.id} value={organization.id}>
									<div className="flex items-center gap-2">
										<Building2 className="size-4" />
										<span>{organization.name}</span>
									</div>
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>

				{selectedOrgId ? (
					<OrganizationLicensePanel
						productId={currentProduct.id}
						organizationId={selectedOrgId}
					/>
				) : (
					<div className="flex flex-col items-center justify-center rounded-lg border border-border bg-muted p-12 text-center">
						<BadgeCheck className="mb-4 size-10 text-muted-foreground" />
						<h3 className="text-lg font-medium">No Organization Selected</h3>
						<p className="mt-1 max-w-sm text-sm text-muted-foreground">
							Select an organization to review its license.
						</p>
					</div>
				)}
			</div>
		</Page>
	);
}
