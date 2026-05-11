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
			description="Update product details"
			variant="full"
		>
			<div className="space-y-6">
				<div className="flex items-center space-x-4">
					<Button onClick={handleBack} variant="outline" size="sm">
						<ArrowLeft className="mr-2 h-4 w-4" />
						Back to Products
					</Button>
				</div>

				<Card className="max-w-2xl">
					<CardHeader>
						<CardTitle>Product Details</CardTitle>
						<CardDescription>
							Update the product information below. All changes will be saved
							immediately.
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
