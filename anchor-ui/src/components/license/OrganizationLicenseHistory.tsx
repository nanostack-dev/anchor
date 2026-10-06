import {
	type OrganizationLicenseHistoryResponse,
	getOrganizationLicenseHistory,
} from "@/client";
import {
	getOrganizationLicenseHistoryQueryKey,
	listLicenseTemplatesOptions,
} from "@/client/@tanstack/react-query.gen";
import { getErrorDetail } from "@/lib/api-error";
import { isHttpQueryError, unwrapQuery } from "@/lib/http-query-error";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useCallback } from "react";
import { OrganizationLicenseHistoryView } from "./OrganizationLicenseHistoryView";

const historyPageSize = 50;

interface OrganizationLicenseHistoryProps {
	productId: string;
	organizationId: string;
}

export function OrganizationLicenseHistory({
	productId,
	organizationId,
}: OrganizationLicenseHistoryProps) {
	const templatesQuery = useQuery(
		listLicenseTemplatesOptions({ path: { product_id: productId } }),
	);
	const templateName = useCallback(
		(templateId: string) =>
			templatesQuery.data?.items?.find((item) => item.id === templateId)
				?.name ?? templateId,
		[templatesQuery.data],
	);

	const historyQuery = useInfiniteQuery({
		queryKey: getOrganizationLicenseHistoryQueryKey({
			path: { product_id: productId, organization_id: organizationId },
			query: { limit: historyPageSize, offset: 0 },
		}),
		initialPageParam: 0,
		getNextPageParam: (
			lastPage: OrganizationLicenseHistoryResponse,
			pages: OrganizationLicenseHistoryResponse[],
		): number | undefined => {
			const loaded = pages.reduce(
				(count, page) => count + page.items.length,
				0,
			);
			return loaded < lastPage.total ? loaded : undefined;
		},
		queryFn: ({ pageParam, signal }) =>
			unwrapQuery(
				getOrganizationLicenseHistory({
					path: { product_id: productId, organization_id: organizationId },
					query: { limit: historyPageSize, offset: pageParam },
					signal,
				}),
			),
		retry: false,
	});

	if (historyQuery.error) {
		const error = historyQuery.error;
		const detail = isHttpQueryError(error)
			? (getErrorDetail(error.body) ??
				`The server responded with HTTP ${error.status}.`)
			: (getErrorDetail(error) ??
				"No response was received at all — the request never reached a server, or a browser-level failure (offline, DNS, CORS) stopped it before one could answer.");

		return (
			<OrganizationLicenseHistoryView
				items={[]}
				total={0}
				errorMessage={detail}
				onRetry={() => void historyQuery.refetch()}
			/>
		);
	}

	return (
		<OrganizationLicenseHistoryView
			items={historyQuery.data?.pages.flatMap((page) => page.items) ?? []}
			templateName={templateName}
			total={historyQuery.data?.pages[0]?.total ?? 0}
			isLoading={historyQuery.isLoading}
			onRetry={() => void historyQuery.refetch()}
			onLoadMore={
				historyQuery.hasNextPage
					? () => void historyQuery.fetchNextPage()
					: undefined
			}
			isLoadingMore={historyQuery.isFetchingNextPage}
		/>
	);
}
