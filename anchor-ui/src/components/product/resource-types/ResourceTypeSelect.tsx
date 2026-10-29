import type { ProductResourceTypeResponse } from "@/client";
import {
	searchProductResourceTypesOptions,
	searchProductResourceTypesQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@nanostackorg/design-system/components/select";
import { Text } from "@nanostackorg/design-system/components/text";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useQuery } from "@tanstack/react-query";

import { type ScopeType, treeOrder } from "./resource-type-scope";

// A product has at most 50 resource types besides organization, so one page
// of 100 holds every type.
const RESOURCE_TYPE_PAGE = 100;

const resourceTypesSearch = (productId: string) => ({
	path: { product_id: productId },
	body: { pagination: { limit: RESOURCE_TYPE_PAGE } },
});

export function resourceTypesQueryKey(productId: string) {
	return searchProductResourceTypesQueryKey(resourceTypesSearch(productId));
}

export function useResourceTypes(productId: string) {
	return useQuery({
		...searchProductResourceTypesOptions(resourceTypesSearch(productId)),
		select: (data): ProductResourceTypeResponse[] => data.items,
	});
}

/** The product's resource types in tree order: Organization first, then top down. */
export function useScopeTypes(productId: string) {
	return useQuery({
		...searchProductResourceTypesOptions(resourceTypesSearch(productId)),
		select: (data): ScopeType[] => treeOrder(data.items),
	});
}

interface ResourceTypeSelectProps {
	id: string;
	productId: string;
	value: string;
	onChange: (slug: string) => void;
	disabled?: boolean;
	invalid?: boolean;
	placeholder?: string;
}

export function ResourceTypeSelect({
	id,
	productId,
	value,
	onChange,
	disabled,
	invalid,
	placeholder = "Choose a resource type",
}: ResourceTypeSelectProps) {
	const {
		data: types = [],
		isPending,
		isError,
		refetch,
	} = useScopeTypes(productId);
	const items = types.map((type) => ({ value: type.slug, label: type.name }));

	return (
		<Stack space="xs">
			<Select
				items={items}
				value={value || null}
				onValueChange={(next) => next && onChange(next)}
				disabled={disabled || isPending || isError}
			>
				<SelectTrigger id={id} width="fill" aria-invalid={invalid || undefined}>
					<SelectValue
						placeholder={isError ? "Couldn't load resource types" : placeholder}
					/>
				</SelectTrigger>
				<SelectContent aria-label="Resource types">
					{items.map((item) => (
						<SelectItem key={item.value} value={item.value}>
							{item.label}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
			{isError && (
				<Stack space="xs" align="start">
					<Text tone="critical" size="sm">
						Anchor didn't return the resource types.
					</Text>
					<Button size="sm" variant="outline" onClick={() => refetch()}>
						Try again
					</Button>
				</Stack>
			)}
		</Stack>
	);
}
