import { useSuspenseQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";

import { getProductOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { ProductEditForm } from "@/components/product/ProductEditForm";
import { productEditRoute } from "@/routes/products/$productId.edit";
import { productsRoute } from "@/routes/products/products";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useNavigate } from "@tanstack/react-router";

export default function ProductEditPage() {
	const { productId } = productEditRoute.useParams();
	const navigate = useNavigate();

	const productQuery = useSuspenseQuery(
		getProductOptions({
			path: { product_id: productId },
		}),
	);
	const product = productQuery.data;

	const handleBack = () => {
		navigate({ to: productsRoute.fullPath });
	};

	const handleSuccess = () => {
		navigate({ to: productsRoute.fullPath });
	};

	return (
		<Page
			title="Edit Product"
			description="Update product details and configuration"
			variant="full"
		>
			<Stack space="lg">
				<Inline space="md">
					<Button onClick={handleBack} variant="outline" size="sm">
						<ArrowLeft data-icon="inline-start" />
						Back to Products
					</Button>
				</Inline>

				<Card>
					<CardHeader>
						<CardTitle>Product</CardTitle>
						<CardDescription>
							Update product information and configuration.
						</CardDescription>
					</CardHeader>
					<CardContent>
						<ProductEditForm
							product={product}
							productId={productId}
							onSuccess={handleSuccess}
							onCancel={handleBack}
						/>
					</CardContent>
				</Card>
			</Stack>
		</Page>
	);
}
