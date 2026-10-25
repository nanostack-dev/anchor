import { Page } from "@/components/common/Page";
import { CreateProductResourcePermissionForm } from "@/components/product/permissions/CreateProductResourcePermissionForm";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { ArrowLeftIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

interface ProductResourcePermissionNewPageProps {
	resourceType?: string;
}

export default function ProductResourcePermissionNewPage({
	resourceType,
}: ProductResourcePermissionNewPageProps) {
	const { currentProduct } = useProduct();
	const navigate = useNavigate();
	return (
		<Page
			variant="default"
			breadCrumbs={false}
			title="New permission"
			description="Choose the resource type it applies to first, then name it."
			actions={
				<ButtonLink
					href={ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS}
					icon={ArrowLeftIcon}
					variant="outline"
					tone="neutral"
				>
					All resource permissions
				</ButtonLink>
			}
		>
			{currentProduct ? (
				<CreateProductResourcePermissionForm
					key={currentProduct.id}
					productId={currentProduct.id}
					initialResourceType={resourceType}
					onCancel={() =>
						void navigate({ to: ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS })
					}
					onCreated={(permission) =>
						void navigate({
							to: ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_DETAIL,
							params: { permissionName: permission.name },
							search: {},
						})
					}
				/>
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>Select a product</EmptyTitle>
						<EmptyDescription>
							Please select a product to create a permission.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
