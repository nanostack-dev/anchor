import { useQuery } from "@tanstack/react-query";
import { Building2, KeyRound } from "lucide-react";
import { useState } from "react";

import { searchProductOrganizationsOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { OrganizationApiKeyDatatable } from "@/components/organization/apikey/OrganizationApiKeyDatatable";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { useProduct } from "@/context/product/ProductContext";

export default function OrganizationApiKeysPage() {
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
						Please select a product to view organization API keys.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page
			title="Organization API Keys"
			description="View API keys issued across organizations in the selected product."
		>
			<div className="flex flex-col gap-6">
				<div className="flex w-full max-w-sm flex-col gap-2">
					<label
						htmlFor="organization-api-key-org-select"
						className="text-sm font-medium leading-none"
					>
						Organization
					</label>
					<Select
						value={selectedOrgId}
						onValueChange={setSelectedOrgId}
						disabled={isLoading || organizations.length === 0}
					>
						<SelectTrigger id="organization-api-key-org-select">
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
					<OrganizationApiKeyDatatable organizationId={selectedOrgId} />
				) : (
					<div className="flex flex-col items-center justify-center rounded-lg border border-border bg-muted p-12 text-center">
						<KeyRound className="mb-4 size-10 text-muted-foreground" />
						<h3 className="text-lg font-medium">No Organization Selected</h3>
						<p className="mt-1 max-w-sm text-sm text-muted-foreground">
							Select an organization to review its API keys and granted Anchor
							permissions.
						</p>
					</div>
				)}
			</div>
		</Page>
	);
}
