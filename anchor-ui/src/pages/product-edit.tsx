import { useSuspenseQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";

import { getProductOptions } from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { ProductEditForm } from "@/components/product/ProductEditForm";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { productEditRoute } from "@/routes/products/$productId.edit";
import { productsRoute } from "@/routes/products/products";
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
			<div className="flex flex-col gap-6">
				<div className="flex items-center gap-4">
					<Button onClick={handleBack} variant="outline" size="sm">
						<ArrowLeft data-icon="inline-start" />
						Back to Products
					</Button>
				</div>

				<Card className="max-w-3xl">
					<CardHeader>
						<CardTitle>Product</CardTitle>
						<CardDescription>
							Update product information and API key generation settings.
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
			</div>
		</Page>
	);
}
