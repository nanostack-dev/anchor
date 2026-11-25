import type { ProductRoleResponse } from "@/client";
import { Page } from "@/components/common/Page";
import { useScopeTypes } from "@/components/product/resource-types/ResourceTypeSelect";
import { typeName } from "@/components/product/resource-types/resource-type-scope";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Button } from "@nanostackorg/design-system/components/button";
import { Card, CardContent } from "@nanostackorg/design-system/components/card";
import { Box } from "@nanostackorg/design-system/layout/box";
import dayjs from "dayjs";
import { ArrowLeft, PenLine } from "lucide-react";
import type { ReactNode } from "react";
import { ProductRoleEditor } from "./ProductRoleEditor";

interface ProductRoleDetailViewProps {
	productId: string;
	role: ProductRoleResponse;
	editing: boolean;
	onCancel: () => void;
	onSaved: () => void;
}

export function ProductRoleDetailView({
	productId,
	role,
	editing,
	onCancel,
	onSaved,
}: ProductRoleDetailViewProps) {
	const scopeName = typeName(
		useScopeTypes(productId).data ?? [],
		role.resource_type,
	);
	return (
		<Page
			variant="default"
			breadCrumbs={false}
			title={editing ? `Edit ${role.name}` : role.name}
			description={role.description || "No description."}
			actions={
				<>
					<ButtonLink
						href={ROUTE_PATHS.PRODUCT_ROLES}
						variant="outline"
						tone="neutral"
					>
						<ArrowLeft data-icon="inline-start" />
						All roles
					</ButtonLink>
					{editing && (
						<Button variant="outline" onClick={onCancel}>
							Cancel editing
						</Button>
					)}
					{!editing && (
						<ButtonLink
							href={`${ROUTE_PATHS.PRODUCT_ROLE_DETAIL.replace(
								"$roleId",
								encodeURIComponent(role.id),
							)}?${new URLSearchParams({ edit: "true" }).toString()}`}
							variant="solid"
							tone="brand"
						>
							<PenLine data-icon="inline-start" />
							Edit role
						</ButtonLink>
					)}
				</>
			}
		>
			{editing ? (
				<ProductRoleEditor
					key={role.id}
					productId={productId}
					mode="edit"
					existingRole={role}
					onCancel={onCancel}
					onSaved={onSaved}
				/>
			) : (
				<Box as="section" aria-label="Role details">
					<Card>
						<CardContent>
							<dl className="grid gap-x-8 gap-y-5 sm:grid-cols-[10rem_1fr]">
								<DetailRow label="Name">
									<Box as="span" className="font-medium">
										{role.name}
									</Box>
								</DetailRow>
								<DetailRow label="Resource type">{scopeName}</DetailRow>
								<DetailRow label="Description">
									{role.description || "No description"}
								</DetailRow>
								<DetailRow label={`Permissions (${role.permissions.length})`}>
									{role.permissions.length ? (
										<Box as="ul" className="flex flex-wrap gap-2">
											{role.permissions.map((permission) => (
												<Box as="li" key={permission.permission_name}>
													<Badge variant="outline">
														{permission.permission_name}
													</Badge>
												</Box>
											))}
										</Box>
									) : (
										"No permissions assigned"
									)}
								</DetailRow>
								<DetailRow label="Created">
									{dayjs(role.created_at).format("D MMMM YYYY H:mm")}
								</DetailRow>
								<DetailRow label="Updated">
									{dayjs(role.updated_at).format("D MMMM YYYY H:mm")}
								</DetailRow>
							</dl>
						</CardContent>
					</Card>
				</Box>
			)}
		</Page>
	);
}

function DetailRow({
	label,
	children,
}: {
	label: string;
	children: ReactNode;
}) {
	return (
		<>
			<dt className="text-sm text-muted-foreground">{label}</dt>
			<dd className="[overflow-wrap:anywhere]">{children}</dd>
		</>
	);
}
