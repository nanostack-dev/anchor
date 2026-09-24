import { getProductRoleOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { ProductRoleDetailView } from "@/components/product/roles/ProductRoleDetailView";
import { Button, buttonVariants } from "@/components/ui/button";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useQuery } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";

interface ProductRoleDetailPageProps {
	roleId: string;
	editing: boolean;
}

export default function ProductRoleDetailPage({
	roleId,
	editing,
}: ProductRoleDetailPageProps) {
	const { currentProduct } = useProduct();
	const productId = currentProduct?.id ?? "";
	const navigate = useNavigate();
	const roleQuery = useQuery({
		...getProductRoleOptions({
			path: { product_id: productId, role_id: roleId },
		}),
		enabled: !!productId,
	});
	const viewRole = () =>
		void navigate({
			to: ROUTE_PATHS.PRODUCT_ROLE_DETAIL,
			params: { roleId },
			search: {},
			replace: true,
		});

	if (
		!currentProduct ||
		roleQuery.isPending ||
		roleQuery.isError ||
		!roleQuery.data
	) {
		return (
			<Page
				variant="default"
				breadCrumbs={false}
				title={
					!currentProduct
						? "Select a product"
						: roleQuery.isPending
							? "Loading role"
							: "Could not load role"
				}
				actions={
					<Link
						to={ROUTE_PATHS.PRODUCT_ROLES}
						className={buttonVariants({ variant: "outline" })}
					>
						All roles
					</Link>
				}
			>
				{roleQuery.isError && (
					<Button onClick={() => void roleQuery.refetch()}>Try again</Button>
				)}
			</Page>
		);
	}

	return (
		<ProductRoleDetailView
			productId={productId}
			role={roleQuery.data}
			editing={editing}
			onCancel={viewRole}
			onSaved={() => {
				void roleQuery.refetch();
				viewRole();
			}}
		/>
	);
}
