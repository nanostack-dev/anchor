import {
	archiveStripeBillingPrice,
	changeStripeBillingSubscription,
	createStripeBillingCheckout,
	createStripeBillingPortal,
	createStripeBillingPrice,
	getStripeBillingState,
	setStripeBillingCancellation,
	syncStripeBillingOrganization,
	updateStripeBillingSettings,
	zApiErrorResponse,
} from "@/client";
import type {
	CheckoutRequest,
	CreatePriceRequest,
	Organization,
	Price,
	Settings,
	State,
	SubscriptionRequest,
	UpdateSettingsRequest,
} from "./billing-types";
import { zState, zUrlResponse } from "./billing-types";

export interface BillingAPI {
	load(): Promise<State>;
	createPrice(body: CreatePriceRequest): Promise<Price>;
	archivePrice(priceId: string): Promise<Price>;
	updateSettings(body: UpdateSettingsRequest): Promise<Settings>;
	checkout(organizationId: string, body: CheckoutRequest): Promise<string>;
	changeSubscription(
		organizationId: string,
		body: SubscriptionRequest,
	): Promise<Organization>;
	cancel(organizationId: string): Promise<Organization>;
	resume(organizationId: string): Promise<Organization>;
	portal(organizationId: string): Promise<string>;
	sync(organizationId: string): Promise<Organization>;
}

export function createBillingAPI(productId: string): BillingAPI {
	const productPath = { product_id: productId };
	const organizationPath = (organizationId: string) => ({
		...productPath,
		organization_id: organizationId,
	});
	return {
		load: async () => {
			const data = (
				await getStripeBillingState({ path: productPath, throwOnError: true })
			).data;
			zState.parse(data);
			return data;
		},
		createPrice: async (body) =>
			(
				await createStripeBillingPrice({
					path: productPath,
					body,
					throwOnError: true,
				})
			).data,
		archivePrice: async (priceId) =>
			(
				await archiveStripeBillingPrice({
					path: { ...productPath, price_id: priceId },
					throwOnError: true,
				})
			).data,
		updateSettings: async (body) =>
			(
				await updateStripeBillingSettings({
					path: productPath,
					body,
					throwOnError: true,
				})
			).data,
		checkout: async (organizationId, body) =>
			zUrlResponse.parse(
				(
					await createStripeBillingCheckout({
						path: organizationPath(organizationId),
						body,
						throwOnError: true,
					})
				).data,
			).url,
		changeSubscription: async (organizationId, body) =>
			(
				await changeStripeBillingSubscription({
					path: organizationPath(organizationId),
					body,
					throwOnError: true,
				})
			).data,
		cancel: async (organizationId) =>
			(
				await setStripeBillingCancellation({
					path: organizationPath(organizationId),
					body: { cancel_at_period_end: true },
					throwOnError: true,
				})
			).data,
		resume: async (organizationId) =>
			(
				await setStripeBillingCancellation({
					path: organizationPath(organizationId),
					body: { cancel_at_period_end: false },
					throwOnError: true,
				})
			).data,
		portal: async (organizationId) =>
			zUrlResponse.parse(
				(
					await createStripeBillingPortal({
						path: organizationPath(organizationId),
						throwOnError: true,
					})
				).data,
			).url,
		sync: async (organizationId) =>
			(
				await syncStripeBillingOrganization({
					path: organizationPath(organizationId),
					throwOnError: true,
				})
			).data,
	};
}

export function billingError(error: unknown): string {
	const parsed = zApiErrorResponse.safeParse(error);
	if (parsed.success)
		return parsed.data.errors.map((item) => item.message).join(" ");
	if (error && typeof error === "object" && "error" in error)
		return String(error.error);
	if (error instanceof Error) return error.message;
	return "The request did not complete. Reload the state and try again.";
}
