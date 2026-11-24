import type { ProductResourcePermissionResponse } from "@/client";
import {
	getProductResourcePermissionQueryKey,
	searchProductResourcePermissionsQueryKey,
	updateProductResourcePermissionMutation,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { useScopeTypes } from "@/components/product/resource-types/ResourceTypeSelect";
import { typeName } from "@/components/product/resource-types/resource-type-scope";
import { getApiErrorMessage } from "@/lib/api-error";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Button } from "@nanostackorg/design-system/components/button";
import { Card, CardContent } from "@nanostackorg/design-system/components/card";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import dayjs from "dayjs";
import { ArrowLeft, PenLine } from "lucide-react";
import { type ReactNode, useState } from "react";

interface ProductResourcePermissionDetailViewProps {
	productId: string;
	permission: ProductResourcePermissionResponse;
	editing: boolean;
	onCancel: () => void;
	onSaved: () => void;
}

export function ProductResourcePermissionDetailView({
	productId,
	permission,
	editing,
	onCancel,
	onSaved,
}: ProductResourcePermissionDetailViewProps) {
	const queryClient = useQueryClient();
	const scopeName = typeName(
		useScopeTypes(productId).data ?? [],
		permission.resource_type,
	);
	const [description, setDescription] = useState(permission.description ?? "");
	const updateMutation = useMutation({
		...updateProductResourcePermissionMutation(),
		onSuccess: () => {
			void queryClient.invalidateQueries({
				queryKey: searchProductResourcePermissionsQueryKey({
					path: { product_id: productId },
					body: {},
				}),
			});
			void queryClient.invalidateQueries({
				queryKey: getProductResourcePermissionQueryKey({
					path: { product_id: productId, permission_name: permission.name },
				}),
			});
			toast.add({ type: "success", title: "Resource permission updated" });
			onSaved();
		},
		onError: (error) => {
			toast.add({
				type: "error",
				title: getApiErrorMessage(error) || "Could not update permission",
			});
		},
	});

	return (
		<Page
			variant="default"
			breadCrumbs={false}
			title={editing ? `Edit ${permission.name}` : permission.name}
			description={permission.description || "No description."}
			actions={
				<>
					<ButtonLink
						href={ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS}
						variant="outline"
						tone="neutral"
					>
						<ArrowLeft data-icon="inline-start" />
						All resource permissions
					</ButtonLink>
					{!editing && (
						<ButtonLink
							href={`${ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_DETAIL.replace(
								"$permissionName",
								encodeURIComponent(permission.name),
							)}?${new URLSearchParams({ edit: "true" }).toString()}`}
							variant="solid"
							tone="brand"
						>
							<PenLine data-icon="inline-start" />
							Edit permission
						</ButtonLink>
					)}
				</>
			}
		>
			{editing ? (
				<Box
					as="form"
					onSubmit={(event) => {
						event.preventDefault();
						updateMutation.mutate({
							path: { product_id: productId, permission_name: permission.name },
							body: { description },
						});
					}}
				>
					<Card>
						<CardContent>
							<Box className="space-y-6">
								<Box className="space-y-2">
									<Label htmlFor="permission-name">Name</Label>
									<Input
										id="permission-name"
										value={permission.name}
										readOnly
										disabled
									/>
									<Box as="p" className="text-sm text-muted-foreground">
										The permission name cannot be changed.
									</Box>
								</Box>
								<Box className="space-y-2">
									<Label htmlFor="permission-resource-type">
										Resource type
									</Label>
									<Input
										id="permission-resource-type"
										value={scopeName}
										readOnly
										disabled
									/>
									<Box as="p" className="text-sm text-muted-foreground">
										A permission's resource type never changes.
									</Box>
								</Box>
								<Box className="space-y-2">
									<Label htmlFor="permission-description">Description</Label>
									<Textarea
										id="permission-description"
										value={description}
										onChange={(event) => setDescription(event.target.value)}
										rows={5}
									/>
								</Box>
								{updateMutation.isError && (
									<Box as="p" role="alert" className="text-sm text-destructive">
										{getApiErrorMessage(updateMutation.error) ||
											"Could not update permission. Try again."}
									</Box>
								)}
								<Box className="flex gap-2">
									<Button
										variant="solid"
										tone="brand"
										type="submit"
										disabled={updateMutation.isPending}
									>
										{updateMutation.isPending ? "Saving..." : "Save changes"}
									</Button>
									<Button type="button" variant="outline" onClick={onCancel}>
										Cancel
									</Button>
								</Box>
							</Box>
						</CardContent>
					</Card>
				</Box>
			) : (
				<Box as="section" aria-label="Resource permission details">
					<Card>
						<CardContent>
							<dl className="grid gap-x-8 gap-y-5 sm:grid-cols-[10rem_1fr]">
								<DetailRow label="Name">
									<Box as="span" className="font-medium">
										{permission.name}
									</Box>
								</DetailRow>
								<DetailRow label="Resource type">{scopeName}</DetailRow>
								<DetailRow label="Description">
									{permission.description || "No description"}
								</DetailRow>
								<DetailRow label="Created">
									{dayjs(permission.created_at).format("D MMMM YYYY H:mm")}
								</DetailRow>
								<DetailRow label="Updated">
									{dayjs(permission.updated_at).format("D MMMM YYYY H:mm")}
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
