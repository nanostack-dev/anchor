import {
	WebhookDeliveryStatus,
	WebhookEndpointStatus,
	type WebhookEventTypeDescriptor,
} from "@/client";
import type { StatusTone } from "@/components/common/StatusBadge";
import dayjs from "dayjs";

/**
 * Endpoint status tones. AUTO_DISABLED is a warning rather than a plain
 * destructive: it is Anchor's own decision after sustained failures and is the
 * state an administrator most needs to notice and act on.
 */
export const endpointStatusTone: Record<WebhookEndpointStatus, StatusTone> = {
	[WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_ENABLED]: "success",
	[WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_DISABLED]: "neutral",
	[WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_AUTODISABLED]: "warning",
};

export const endpointStatusLabel: Record<WebhookEndpointStatus, string> = {
	[WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_ENABLED]: "Enabled",
	[WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_DISABLED]: "Disabled",
	[WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_AUTODISABLED]: "Auto-disabled",
};

export const deliveryStatusTone: Record<WebhookDeliveryStatus, StatusTone> = {
	[WebhookDeliveryStatus.WEBHOOK_DELIVERY_STATUS_PENDING]: "warning",
	[WebhookDeliveryStatus.WEBHOOK_DELIVERY_STATUS_SUCCEEDED]: "success",
	[WebhookDeliveryStatus.WEBHOOK_DELIVERY_STATUS_FAILED]: "destructive",
	[WebhookDeliveryStatus.WEBHOOK_DELIVERY_STATUS_EXHAUSTED]: "destructive",
};

export const deliveryStatusOptions: Array<{ label: string; value: string }> = [
	{ label: "Pending", value: "PENDING" },
	{ label: "Succeeded", value: "SUCCEEDED" },
	{ label: "Failed", value: "FAILED" },
	{ label: "Exhausted", value: "EXHAUSTED" },
];

export const WILDCARD_SUFFIX = ".*";

export const formatDateTime = (value: string | null | undefined): string =>
	value ? dayjs(value).format("D MMMM YYYY H:mm") : "—";

export const formatDuration = (durationMs: number): string =>
	durationMs < 1000
		? `${durationMs} ms`
		: `${(durationMs / 1000).toFixed(2)} s`;

export interface EventTypeGroup {
	group: string;
	descriptors: WebhookEventTypeDescriptor[];
	/**
	 * A `<group>.*` subscription only makes sense for groups whose types follow
	 * the `<group>.<event>` grammar. `ping` is its own group *and* its own type,
	 * so offering "all ping events" would be noise.
	 */
	wildcard: string | null;
}

/** Groups the registry catalog for presentation, preserving catalog order. */
export function groupEventTypes(
	descriptors: WebhookEventTypeDescriptor[],
): EventTypeGroup[] {
	const groups: EventTypeGroup[] = [];
	for (const descriptor of descriptors) {
		let entry = groups.find(
			(candidate) => candidate.group === descriptor.group,
		);
		if (!entry) {
			entry = { group: descriptor.group, descriptors: [], wildcard: null };
			groups.push(entry);
		}
		entry.descriptors.push(descriptor);
		if (descriptor.type !== descriptor.group) {
			entry.wildcard = `${descriptor.group}${WILDCARD_SUFFIX}`;
		}
	}
	return groups;
}

/**
 * Mirrors the server-side subscription matcher: an exact type, or a
 * `<group>.*` wildcard over the type's group.
 */
export function isEventTypeCovered(
	subscriptions: string[],
	descriptor: WebhookEventTypeDescriptor,
): boolean {
	return subscriptions.some(
		(subscription) =>
			subscription === descriptor.type ||
			subscription === `${descriptor.group}${WILDCARD_SUFFIX}`,
	);
}
