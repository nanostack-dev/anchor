import { getOrganizationLicense } from "@/client";
import { getOrganizationLicenseQueryKey } from "@/client/@tanstack/react-query.gen";
import { unwrapQuery } from "@/lib/http-query-error";
import { useQuery } from "@tanstack/react-query";

/**
 * Raw (non-throwing) SDK call, not getOrganizationLicenseOptions(): see the
 * matching comment in LicenseSchemaPanel — the *Options() helper's
 * throwOnError mode loses the HTTP status once a response body doesn't parse
 * as an ApiErrorResponse, which an empty or plain-text 404 body from an
 * earlier middleware layer will do.
 *
 * Every tab of the license detail route calls this. They share one cache
 * entry, so the license is fetched once and each tab reads it.
 */
export function useOrganizationLicenseQuery(
	productId: string | undefined,
	organizationId: string,
) {
	const path = {
		product_id: productId as string,
		organization_id: organizationId,
	};

	return useQuery({
		queryKey: getOrganizationLicenseQueryKey({ path }),
		queryFn: () => unwrapQuery(getOrganizationLicense({ path })),
		enabled: !!productId,
		retry: false,
	});
}
