import type { ProductRoleResponse } from "@/client";
import { Page } from "@/components/common/Page";
import { useScopeTypes } from "@/components/product/resource-types/ResourceTypeSelect";
import { typeName } from "@/components/product/resource-types/resource-type-scope";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Button } from "@nanostackorg/design-system/components/button";
import { Box } from "@nanostackorg/design-system/layout/box";
import dayjs from "dayjs";
import { ArrowLeft, PenLine } from "lucide-react";
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
				<Box
					as="section"
					className="rounded-lg border border-border p-6"
					aria-label="Role details"
				>
					<dl className="grid gap-6 sm:grid-cols-2">
						<div>
							<dt className="text-sm text-muted-foreground">Name</dt>
							<dd className="mt-1 flex flex-wrap items-center gap-2 font-medium [overflow-wrap:anywhere]">
								{role.name}
								{role.is_default && <Badge tone="brand">Default</Badge>}
							</dd>
						</div>
						<div>
							<dt className="text-sm text-muted-foreground">Resource type</dt>
							<dd className="mt-1 [overflow-wrap:anywhere]">{scopeName}</dd>
						</div>
						<div>
							<dt className="text-sm text-muted-foreground">Key</dt>
							<dd className="mt-1 font-mono text-sm [overflow-wrap:anywhere]">
								{role.key}
							</dd>
						</div>
						<div>
							<dt className="text-sm text-muted-foreground">Description</dt>
							<dd className="mt-1">{role.description || "No description"}</dd>
						</div>
						<Box className="sm:col-span-2">
							<dt className="text-sm text-muted-foreground">
								Permissions ({role.permissions.length})
							</dt>
							<dd className="mt-2">
								{role.permissions.length ? (
									<Box as="ul" className="flex flex-wrap gap-2">
										{role.permissions.map((permission) => (
											<Box
												as="li"
												key={permission.permission_name}
												className="rounded-md border border-border px-2 py-1 text-sm"
											>
												{permission.permission_name}
											</Box>
										))}
									</Box>
								) : (
									"No permissions assigned"
								)}
							</dd>
						</Box>
						<div>
							<dt className="text-sm text-muted-foreground">Created</dt>
							<dd className="mt-1">
								{dayjs(role.created_at).format("D MMMM YYYY H:mm")}
							</dd>
						</div>
						<div>
							<dt className="text-sm text-muted-foreground">Updated</dt>
							<dd className="mt-1">
								{dayjs(role.updated_at).format("D MMMM YYYY H:mm")}
							</dd>
						</div>
					</dl>
				</Box>
			)}
		</Page>
	);
}
