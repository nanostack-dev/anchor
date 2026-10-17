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
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
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
		<Box className="flex min-w-0 max-w-full flex-wrap items-center gap-2">
			{currentProduct && !error && products.length > 0 && (
				<Box className="min-w-0 basis-32 grow">
					<DropdownMenu>
						<DropdownMenuTrigger
							aria-label={`Working on: ${currentProduct.name}`}
							title={currentProduct.name}
							render={<Button variant="outline" size="sm" width="fill" />}
						>
							<span
								className="size-2 shrink-0 rounded-full bg-success"
								aria-hidden
							/>
							<Box as="span" className="hidden shrink-0 sm:block">
								<Text as="span" tone="muted">
									Working on:
								</Text>
							</Box>
							<Text as="span" weight="medium" truncate>
								{currentProduct.name}
							</Text>
							<ChevronDownIcon className="text-muted-foreground" />
						</DropdownMenuTrigger>
						<DropdownMenuContent align="start" width="md">
							{products.map((product) => (
								<DropdownMenuItem
									key={product.id}
									onClick={() => selectProduct(product)}
								>
									<Box className="flex min-w-0 flex-1 flex-col">
										<Box className="flex min-w-0 items-center gap-2">
											{currentProduct.id === product.id && (
												<span
													className="size-2 shrink-0 rounded-full bg-success"
													aria-hidden
												/>
											)}
											<Box
												as="span"
												className="min-w-0 wrap-anywhere font-medium"
											>
												{product.name}
											</Box>
										</Box>
										{product.description && (
											<span
												className="mt-1 truncate text-xs text-muted-foreground"
												title={product.description}
											>
												{product.description}
											</span>
										)}
									</Box>
								</DropdownMenuItem>
							))}
						</DropdownMenuContent>
					</DropdownMenu>
				</Box>
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
				<Box className="flex shrink-0 items-center gap-2">
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
				</Box>
			)}
		</Box>
	);
}
