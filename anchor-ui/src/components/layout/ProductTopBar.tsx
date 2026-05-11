import { ProductCreateDialog } from "@/components/product/ProductCreateDialog";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useProduct } from "@/hooks/useProduct";
import { AlertCircle, ChevronDown, RefreshCw, Sparkles } from "lucide-react";

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
		<div className="border-b bg-gradient-to-r from-background to-muted/20 px-6 py-4 shadow-sm">
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-2">
					{currentProduct && !error && products.length > 0 && (
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<button
									type="button"
									className="flex items-center gap-2 bg-muted/30 px-3 py-1.5 rounded-lg border hover:bg-muted/50 transition-colors cursor-pointer"
								>
									<div className="w-2 h-2 bg-green-500 rounded-full animate-pulse" />
									<span className="text-sm text-muted-foreground">
										Working on:
									</span>
									<span className="font-medium text-foreground">
										{currentProduct.name}
									</span>
									<ChevronDown className="h-3 w-3 text-muted-foreground ml-1" />
								</button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="start" className="w-64">
								{products.map((product) => (
									<DropdownMenuItem
										key={product.id}
										onClick={() => selectProduct(product)}
										className={`cursor-pointer ${
											currentProduct.id === product.id
												? "bg-muted font-medium"
												: ""
										}`}
									>
										<div className="flex flex-col w-full">
											<div className="flex items-center gap-2">
												{currentProduct.id === product.id && (
													<div className="w-2 h-2 bg-green-500 rounded-full" />
												)}
												<span className="font-medium">{product.name}</span>
											</div>
											{product.description && (
												<span className="text-xs text-muted-foreground truncate mt-1">
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
						<Alert className="">
							<AlertCircle className="h-4 w-4" />
							<AlertDescription className="text-sm">
								Failed to load products.
								<Button
									variant="link"
									className="p-0 h-auto text-sm underline ml-1"
									onClick={handleRefresh}
								>
									Try again
								</Button>
							</AlertDescription>
						</Alert>
					) : products.length === 0 && !isLoading ? (
						<ProductCreateDialog
							trigger={
								<Button
									size="sm"
									className="bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-700 hover:to-indigo-700 text-white shadow-lg hover:shadow-xl transition-all duration-300 transform hover:scale-105 border-0 font-medium px-4 py-2"
								>
									<Sparkles className="h-4 w-4 mr-2 animate-pulse" />
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
								size="sm"
								onClick={handleRefresh}
								disabled={isLoading}
								className="shrink-0"
							>
								<RefreshCw
									className={`h-4 w-4 ${isLoading ? "animate-spin" : ""}`}
								/>
							</Button>
						</div>
					)}
				</div>
			</div>
		</div>
	);
}
