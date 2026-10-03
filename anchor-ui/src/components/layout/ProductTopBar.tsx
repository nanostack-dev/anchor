import { ProductCreateDialog } from "@/components/product/ProductCreateDialog";
import { useProduct } from "@/hooks/useProduct";
import {
	Alert,
	AlertDescription,
} from "@nanostackorg/design-system/components/alert";
import {
	Button,
	IconButton,
} from "@nanostackorg/design-system/components/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@nanostackorg/design-system/components/dropdown-menu";
import { ArrowsClockwiseIcon } from "@phosphor-icons/react";
import { AlertCircleIcon, ChevronDownIcon, SparklesIcon } from "lucide-react";

export function ProductTopBar() {
	const {
		currentProduct,
		products,
		isLoading,
		error,
		selectProduct,
		refreshProducts,
	} = useProduct();

	const handleRefresh = () => {
		refreshProducts();
	};

	const handleProductCreated = () => {
		// Refresh products list after creation
		refreshProducts();
	};

	return (
		<div className="flex min-w-0 items-center gap-2">
			{currentProduct && !error && products.length > 0 && (
				<DropdownMenu>
					<DropdownMenuTrigger render={<Button variant="outline" size="sm" />}>
						<span
							className="size-2 shrink-0 rounded-full bg-success"
							aria-hidden
						/>
						<span className="text-muted-foreground">Working on:</span>
						<span className="truncate font-medium text-foreground">
							{currentProduct.name}
						</span>
						<ChevronDownIcon className="text-muted-foreground" />
					</DropdownMenuTrigger>
					<DropdownMenuContent align="start">
						{products.map((product) => (
							<DropdownMenuItem
								key={product.id}
								onClick={() => selectProduct(product)}
							>
								<div className="flex w-full flex-col">
									<div className="flex items-center gap-2">
										{currentProduct.id === product.id && (
											<span
												className="size-2 shrink-0 rounded-full bg-success"
												aria-hidden
											/>
										)}
										<span className="font-medium">{product.name}</span>
									</div>
									{product.description && (
										<span className="mt-1 truncate text-xs text-muted-foreground">
											{product.description}
										</span>
									)}
								</div>
							</DropdownMenuItem>
						))}
					</DropdownMenuContent>
				</DropdownMenu>
			)}

			{error ? (
				<Alert tone="critical">
					<AlertCircleIcon />
					<AlertDescription>
						Failed to load products.
						<Button variant="ghost" tone="brand" onClick={handleRefresh}>
							Try again
						</Button>
					</AlertDescription>
				</Alert>
			) : products.length === 0 && !isLoading ? (
				<ProductCreateDialog
					trigger={
						<Button variant="solid" tone="brand" size="sm">
							<SparklesIcon data-icon="inline-start" />
							Create Your First Product
						</Button>
					}
					onCreated={handleProductCreated}
				/>
			) : (
				<div className="flex items-center gap-2">
					{isLoading && (
						<span className="text-sm text-muted-foreground">
							Loading products...
						</span>
					)}
					<IconButton
						variant="outline"
						size="sm"
						icon={ArrowsClockwiseIcon}
						label="Refresh products"
						onClick={handleRefresh}
						loading={isLoading}
					/>
				</div>
			)}
		</div>
	);
}
