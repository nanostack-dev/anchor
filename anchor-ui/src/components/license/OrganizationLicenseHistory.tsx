import {
	type OrganizationLicenseHistoryResponse,
	SortDirection,
	searchOrganizationLicenseHistory,
} from "@/client";
import { searchOrganizationLicenseHistoryQueryKey } from "@/client/@tanstack/react-query.gen";
import { allLicenseTemplatesOptions } from "@/hooks/useAllLicenseTemplates";
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
	const templatesQuery = useQuery(allLicenseTemplatesOptions(productId));
	const templateName = useCallback(
		(templateId: string) =>
			templatesQuery.data?.items?.find((item) => item.id === templateId)
				?.name ?? templateId,
		[templatesQuery.data],
	);

	const historyQuery = useInfiniteQuery({
		queryKey: searchOrganizationLicenseHistoryQueryKey({
			path: { product_id: productId, organization_id: organizationId },
			body: {
				pagination: { limit: historyPageSize },
				sort_by: "changed_at",
				sort_direction: SortDirection.DESC,
			},
		}),
		initialPageParam: undefined as string | undefined,
		getNextPageParam: (
			lastPage: OrganizationLicenseHistoryResponse,
		): string | undefined => lastPage.next_cursor,
		queryFn: ({ pageParam, signal }) =>
			unwrapQuery(
				searchOrganizationLicenseHistory({
					path: { product_id: productId, organization_id: organizationId },
					body: {
						pagination: { limit: historyPageSize, cursor: pageParam },
						sort_by: "changed_at",
						sort_direction: SortDirection.DESC,
					},
					signal,
				}),
			),
		retry: false,
	});

	if (historyQuery.error && !historyQuery.data) {
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
			loadMoreErrorMessage={
				historyQuery.isFetchNextPageError
					? "Couldn’t load older changes. Try again."
					: null
			}
		/>
	);
}
