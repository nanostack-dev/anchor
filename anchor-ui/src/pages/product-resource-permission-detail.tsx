import { getProductResourcePermissionOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { ProductResourcePermissionDetailView } from "@/components/product/permissions/ProductResourcePermissionDetailView";
import { Button, buttonVariants } from "@/components/ui/button";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";

interface ProductResourcePermissionDetailPageProps {
	permissionName: string;
	editing: boolean;
}

export default function ProductResourcePermissionDetailPage({
	permissionName,
	editing,
}: ProductResourcePermissionDetailPageProps) {
	const { currentProduct } = useProduct();
	const productId = currentProduct?.id ?? "";
	const navigate = useNavigate();
	const permissionQuery = useQuery({
		...getProductResourcePermissionOptions({
			path: { product_id: productId, permission_name: permissionName },
		}),
		enabled: !!productId,
	});
	const viewPermission = () =>
		void navigate({
			to: ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_DETAIL,
			params: { permissionName },
			search: {},
			replace: true,
		});

	if (
		!currentProduct ||
		permissionQuery.isPending ||
		permissionQuery.isError ||
		!permissionQuery.data
	) {
		return (
			<Page
				variant="default"
				breadCrumbs={false}
				title={
					!currentProduct
						? "Select a product"
						: permissionQuery.isPending
							? "Loading resource permission"
							: "Could not load resource permission"
				}
				actions={
					<Link
						to={ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS}
						className={buttonVariants({ variant: "outline" })}
					>
						All resource permissions
					</Link>
				}
			>
				{permissionQuery.isError && (
					<Button onClick={() => void permissionQuery.refetch()}>
						Try again
					</Button>
				)}
			</Page>
		);
	}

	return (
		<ProductResourcePermissionDetailView
			key={`${permissionName}:${editing}`}
			productId={productId}
			permission={permissionQuery.data}
			editing={editing}
			onCancel={viewPermission}
			onSaved={() => {
				void permissionQuery.refetch();
				viewPermission();
			}}
		/>
	);
}
