import type { ProductResourceTypeResponse } from "@/client";
import {
	searchProductResourceTypesOptions,
	searchProductResourceTypesQueryKey,
} from "@/client/@tanstack/react-query.gen";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@nanostackorg/design-system/components/select";
import { useQuery } from "@tanstack/react-query";

export const ORGANIZATION_RESOURCE_TYPE = "organization";

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

interface ResourceTypeSelectProps {
	id: string;
	productId: string;
	value: string;
	onChange: (slug: string) => void;
	disabled?: boolean;
}

export function ResourceTypeSelect({
	id,
	productId,
	value,
	onChange,
	disabled,
}: ResourceTypeSelectProps) {
	const { data: types = [], isPending } = useResourceTypes(productId);
	const items = types.length
		? types.map((type) => ({ value: type.slug, label: type.name }))
		: [{ value: ORGANIZATION_RESOURCE_TYPE, label: "Organization" }];

	return (
		<Select
			items={items}
			value={value}
			onValueChange={(next) => next && onChange(next)}
			disabled={disabled || isPending}
		>
			<SelectTrigger id={id} width="fill">
				<SelectValue />
			</SelectTrigger>
			<SelectContent aria-label="Resource types">
				{items.map((item) => (
					<SelectItem key={item.value} value={item.value}>
						{item.label}
					</SelectItem>
				))}
			</SelectContent>
		</Select>
	);
}
