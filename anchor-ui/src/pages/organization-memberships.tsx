import { searchProductOrganizationsOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { OrganizationMembershipDatatable } from "@/components/organization/OrganizationMembershipDatatable";
import { OrganizationInvitationDatatable } from "@/components/organization/invitation/OrganizationInvitationDatatable";
import { useProduct } from "@/context/product/ProductContext";
import { Inline, Stack } from "@nanostackorg/design-system";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";
import { Card, CardContent } from "@nanostackorg/design-system/components/card";
import {
	Field,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@nanostackorg/design-system/components/select";
import {
	Tabs,
	TabsContent,
	TabsList,
	TabsTrigger,
} from "@nanostackorg/design-system/components/tabs";
import { BuildingsIcon } from "@phosphor-icons/react";
import { useQuery } from "@tanstack/react-query";
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
			description="View members and invitations across your product's organizations."
		>
			<Stack space="lg">
				<Card>
					<CardContent>
						<Stack space="lg">
							<Field>
								<FieldLabel htmlFor="org-select">Organization</FieldLabel>
								<Inline>
									<Select
										items={organizations.map((org) => ({
											value: org.id,
											label: org.name,
										}))}
										value={selectedOrgId ?? null}
										onValueChange={(value) => setSelectedOrgId(value ?? "")}
										disabled={isLoading || organizations.length === 0}
									>
										<SelectTrigger id="org-select" width="auto">
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
											{organizations.map((org) => (
												<SelectItem key={org.id} value={org.id}>
													<Inline space="xs">
														<BuildingsIcon />
														<span>{org.name}</span>
													</Inline>
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								</Inline>
							</Field>
							{selectedOrgId ? null : (
								<EmptyState
									icon={BuildingsIcon}
									title="No Organization Selected"
									description="Please select an organization from the dropdown above to view its members."
								/>
							)}
						</Stack>
					</CardContent>
				</Card>

				{selectedOrgId ? (
					<Tabs defaultValue="members">
						<TabsList>
							<TabsTrigger value="members">Members</TabsTrigger>
							<TabsTrigger value="invitations">Invitations</TabsTrigger>
						</TabsList>
						<TabsContent value="members">
							<OrganizationMembershipDatatable organizationId={selectedOrgId} />
						</TabsContent>
						<TabsContent value="invitations">
							<OrganizationInvitationDatatable organizationId={selectedOrgId} />
						</TabsContent>
					</Tabs>
				) : null}
			</Stack>
		</Page>
	);
}
