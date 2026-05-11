import type { Options, ProductResponse, SearchProductsData } from "@/client";
import { SortDirection } from "@/client";
import { searchProductsOptions } from "@/client/@tanstack/react-query.gen";
import { useQuery } from "@tanstack/react-query";
import type React from "react";
import {
	createContext,
	useCallback,
	useContext,
	useEffect,
	useState,
} from "react";

interface ProductContextType {
	currentProduct: ProductResponse | null;
	products: ProductResponse[];
	isLoading: boolean;
	error: Error | null;
	selectProduct: (product: ProductResponse) => void;
	refreshProducts: () => void;
}

const ProductContext = createContext<ProductContextType | undefined>(undefined);

const SELECTED_PRODUCT_KEY = "selectedProductId";

export const ProductProvider: React.FC<{ children: React.ReactNode }> = ({
	children,
}) => {
	const [currentProduct, setCurrentProduct] = useState<ProductResponse | null>(
		null,
	);

	const searchProductsOptionsParams: Options<SearchProductsData> = {
		body: {
			pagination: {
				limit: 100,
				offset: 0,
			},
			sort_by: "name",
			sort_direction: SortDirection.ASC,
		},
	};

	const {
		data: productData,
		isLoading,
		error,
		refetch: refreshProducts,
	} = useQuery({
		...searchProductsOptions(searchProductsOptionsParams),
	});

	const products = productData?.items ?? [];

	const selectProduct = useCallback((product: ProductResponse) => {
		setCurrentProduct(product);
		localStorage.setItem(SELECTED_PRODUCT_KEY, product.id);
	}, []);

	// Auto-selection logic: localStorage → first available → null
	useEffect(() => {
		if (products.length === 0) {
			setCurrentProduct(null);
			return;
		}

		// Check if we have a saved product ID
		const savedProductId = localStorage.getItem(SELECTED_PRODUCT_KEY);

		if (savedProductId) {
			// Try to find the saved product in the current list
			const savedProduct = products.find((p) => p.id === savedProductId);
			if (savedProduct) {
				setCurrentProduct(savedProduct);
				return;
			}
		}

		// Fallback to first product if saved product doesn't exist
		const firstProduct = products[0];
		if (firstProduct) {
			setCurrentProduct(firstProduct);
			localStorage.setItem(SELECTED_PRODUCT_KEY, firstProduct.id);
		}
	}, [products]);

	return (
		<ProductContext.Provider
			value={{
				currentProduct,
				products,
				isLoading,
				error: error as Error | null,
				selectProduct,
				refreshProducts,
			}}
		>
			{children}
		</ProductContext.Provider>
	);
};

export function useProduct() {
	const ctx = useContext(ProductContext);
	if (!ctx) throw new Error("useProduct must be used within a ProductProvider");
	return ctx;
}
