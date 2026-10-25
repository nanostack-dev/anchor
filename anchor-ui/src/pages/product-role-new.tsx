import { Page } from "@/components/common/Page";
import { ProductRoleEditor } from "@/components/product/roles/ProductRoleEditor";
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

export default function ProductRoleNewPage() {
	const { currentProduct } = useProduct();
	const navigate = useNavigate();
	const backLink = (
		<ButtonLink
			href={ROUTE_PATHS.PRODUCT_ROLES}
			icon={ArrowLeftIcon}
			variant="outline"
			tone="neutral"
		>
			All roles
		</ButtonLink>
	);
	return (
		<Page
			variant="default"
			breadCrumbs={false}
			title="New role"
			description="A role template: every organization gets its own copy of it."
			actions={backLink}
		>
			{currentProduct ? (
				<ProductRoleEditor
					key={currentProduct.id}
					productId={currentProduct.id}
					mode="create"
					onCancel={() => void navigate({ to: ROUTE_PATHS.PRODUCT_ROLES })}
					onSaved={(role) =>
						void navigate({
							to: ROUTE_PATHS.PRODUCT_ROLE_DETAIL,
							params: { roleId: role.id },
							search: {},
						})
					}
				/>
			) : (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>Select a product</EmptyTitle>
						<EmptyDescription>
							Please select a product to create a role.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			)}
		</Page>
	);
}
