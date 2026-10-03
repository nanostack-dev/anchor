import {
	Heading,
	Inline,
	Label,
	Stack,
	Text,
} from "@nanostackorg/design-system";
import { useQuery } from "@tanstack/react-query";
import { Building2, PanelsTopLeft } from "lucide-react";
import { useEffect, useState } from "react";

import { searchProductOrganizationsOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { OrganizationWorkspaceDatatable } from "@/components/organization/OrganizationWorkspaceDatatable";
import { useProduct } from "@/context/product/ProductContext";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@nanostackorg/design-system/components/select";

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
				<Stack space="lg" align="center">
					<Text tone="muted">Please select a product to view workspaces.</Text>
				</Stack>
			</Page>
		);
	}

	return (
		<Page
			title="Organization Workspaces"
			description="Read-only workspace visibility for organizations in the selected product."
		>
			<Stack space="lg">
				<Stack space="lg">
					<Label htmlFor="workspace-org-select">Organization</Label>
					<Select
						items={organizations.map((organization) => ({
							value: organization.id,
							label: organization.name,
						}))}
						value={selectedOrgId ?? null}
						onValueChange={(value) => setSelectedOrgId(value ?? "")}
						disabled={isLoading || organizations.length === 0}
					>
						<SelectTrigger id="workspace-org-select" width="fill">
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
						<SelectContent aria-label="Organization options">
							{organizations.map((organization) => (
								<SelectItem key={organization.id} value={organization.id}>
									<Inline space="xs">
										<Building2 />
										<span>{organization.name}</span>
									</Inline>
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</Stack>

				{selectedOrgId ? (
					<OrganizationWorkspaceDatatable organizationId={selectedOrgId} />
				) : (
					<Stack space="lg" align="center">
						<PanelsTopLeft />
						<Heading level={3}>No Organization Selected</Heading>
						<Text tone="muted">
							Select an organization to review its workspaces. This view is
							read-only for platform admins.
						</Text>
					</Stack>
				)}
			</Stack>
		</Page>
	);
}
