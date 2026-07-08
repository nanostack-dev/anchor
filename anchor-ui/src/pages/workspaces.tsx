import { useQuery } from "@tanstack/react-query";
import { Building2, PanelsTopLeft } from "lucide-react";
import { useEffect, useState } from "react";

import { searchProductOrganizationsOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { OrganizationWorkspaceDatatable } from "@/components/organization/OrganizationWorkspaceDatatable";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { useProduct } from "@/context/product/ProductContext";

export default function WorkspacesPage() {
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

	useEffect(() => {
		if (!selectedOrgId) {
			return;
		}

		const hasSelectedOrganization = organizations.some(
			(organization) => organization.id === selectedOrgId,
		);
		if (!hasSelectedOrganization) {
			setSelectedOrgId("");
		}
	}, [organizations, selectedOrgId]);

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex h-64 items-center justify-center">
					<p className="text-muted-foreground">
						Please select a product to view workspaces.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page
			title="Organization Workspaces"
			description="Read-only workspace visibility for organizations in the selected product."
		>
			<div className="flex flex-col gap-6">
				<div className="flex w-full max-w-sm flex-col gap-2">
					<label
						htmlFor="workspace-org-select"
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
						<SelectTrigger id="workspace-org-select">
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
					<OrganizationWorkspaceDatatable organizationId={selectedOrgId} />
				) : (
					<div className="flex flex-col items-center justify-center rounded-lg border border-border bg-muted p-12 text-center">
						<PanelsTopLeft className="mb-4 size-10 text-muted-foreground" />
						<h3 className="text-lg font-medium">No Organization Selected</h3>
						<p className="mt-1 max-w-sm text-sm text-muted-foreground">
							Select an organization to review its workspaces. This view is
							read-only for platform admins.
						</p>
					</div>
				)}
			</div>
		</Page>
	);
}
