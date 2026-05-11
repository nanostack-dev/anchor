import { useProduct as useProductContext } from "@/context/product/ProductContext";

/**
 * Convenience hook to access the Product context.
 * Provides access to current product, product list, and selection methods.
 */
export const useProduct = useProductContext;
