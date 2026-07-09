import { ProductCreateDialog } from "@/components/product/ProductCreateDialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { useProduct } from "@/hooks/useProduct";
import {
	AlertCircleIcon,
	ChevronDownIcon,
	RefreshCwIcon,
	SparklesIcon,
} from "lucide-react";

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
					<DropdownMenuTrigger
						render={<Button variant="outline" size="sm" className="min-w-0" />}
					>
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
					<DropdownMenuContent align="start" className="w-64">
						{products.map((product) => (
							<DropdownMenuItem
								key={product.id}
								onClick={() => selectProduct(product)}
								className={
									currentProduct.id === product.id ? "bg-muted font-medium" : ""
								}
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
				<Alert variant="destructive" className="py-2">
					<AlertCircleIcon />
					<AlertDescription>
						Failed to load products.
						<Button
							variant="link"
							className="ml-1 h-auto p-0 text-sm underline"
							onClick={handleRefresh}
						>
							Try again
						</Button>
					</AlertDescription>
				</Alert>
			) : products.length === 0 && !isLoading ? (
				<ProductCreateDialog
					trigger={
						<Button size="sm">
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
					<Button
						variant="outline"
						size="icon-sm"
						onClick={handleRefresh}
						disabled={isLoading}
						aria-label="Refresh products"
					>
						{isLoading ? <Spinner /> : <RefreshCwIcon />}
					</Button>
				</div>
			)}
		</div>
	);
}
