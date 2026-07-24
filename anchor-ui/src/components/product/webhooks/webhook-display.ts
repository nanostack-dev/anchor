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

/**
 * A delivery in a terminal state will never change again, which is what stops
 * the test-event panel polling.
 */
export function isTerminalDeliveryStatus(
	status: WebhookDeliveryStatus,
): boolean {
	return status !== WebhookDeliveryStatus.WEBHOOK_DELIVERY_STATUS_PENDING;
}

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

/**
 * How much of a group a subscription list covers. `partial` is what drives the
 * indeterminate state of the group checkbox.
 */
export type GroupSelection = "none" | "partial" | "all";

export function groupSelection(
	subscriptions: string[],
	group: EventTypeGroup,
): GroupSelection {
	const covered = group.descriptors.filter((descriptor) =>
		isEventTypeCovered(subscriptions, descriptor),
	).length;
	if (covered === 0) return "none";
	return covered === group.descriptors.length ? "all" : "partial";
}

/**
 * The shortest subscription entries that cover a whole group: the `<group>.*`
 * wildcard where the grammar allows one, otherwise the group's types spelled
 * out. Preferring the wildcard is not just brevity — it is what keeps the
 * endpoint subscribed to types added to the group later.
 */
export function groupSubscription(group: EventTypeGroup): string[] {
	return group.wildcard
		? [group.wildcard]
		: group.descriptors.map((descriptor) => descriptor.type);
}

/** Every subscription entry needed to cover the whole catalog. */
export function allSubscriptions(groups: EventTypeGroup[]): string[] {
	return groups.flatMap(groupSubscription);
}

/** Drops every entry of a group: its wildcard and its exact types. */
function withoutGroup(
	subscriptions: string[],
	group: EventTypeGroup,
): string[] {
	return subscriptions.filter(
		(entry) =>
			entry !== group.wildcard &&
			!group.descriptors.some((descriptor) => descriptor.type === entry),
	);
}

const unique = (entries: string[]): string[] => [...new Set(entries)];

/**
 * Collapses a fully selected group back to its wildcard, so saving a whole
 * group also subscribes to types added to it later — and so the saved value
 * round-trips to the same checkbox state it was drawn from.
 */
function collapseGroup(
	subscriptions: string[],
	group: EventTypeGroup,
): string[] {
	if (!group.wildcard) return subscriptions;
	if (groupSelection(subscriptions, group) !== "all") return subscriptions;
	return [...withoutGroup(subscriptions, group), group.wildcard];
}

/** Selects or clears an entire group. */
export function toggleGroup(
	subscriptions: string[],
	group: EventTypeGroup,
): string[] {
	const rest = withoutGroup(subscriptions, group);
	return groupSelection(subscriptions, group) === "all"
		? rest
		: unique([...rest, ...groupSubscription(group)]);
}

/**
 * Selects or clears one event type.
 *
 * Clearing a type covered by a wildcard first expands that wildcard into the
 * exact types it stands for; otherwise unchecking one box would silently
 * unsubscribe the whole group.
 */
export function toggleEventType(
	subscriptions: string[],
	group: EventTypeGroup,
	descriptor: WebhookEventTypeDescriptor,
): string[] {
	const covered = isEventTypeCovered(subscriptions, descriptor);
	const expanded =
		group.wildcard && subscriptions.includes(group.wildcard)
			? unique([
					...subscriptions.filter((entry) => entry !== group.wildcard),
					...group.descriptors.map((entry) => entry.type),
				])
			: subscriptions;

	const next = covered
		? expanded.filter((entry) => entry !== descriptor.type)
		: unique([...expanded, descriptor.type]);

	return collapseGroup(next, group);
}

/** How many catalog event types a subscription list actually covers. */
export function coveredEventTypeCount(
	subscriptions: string[],
	descriptors: WebhookEventTypeDescriptor[],
): number {
	return descriptors.filter((descriptor) =>
		isEventTypeCovered(subscriptions, descriptor),
	).length;
}

/** Matches the filter box against the type, its group and its description. */
export function matchesEventTypeQuery(
	descriptor: WebhookEventTypeDescriptor,
	query: string,
): boolean {
	const needle = query.trim().toLowerCase();
	if (needle === "") return true;

	return (
		descriptor.type.toLowerCase().includes(needle) ||
		descriptor.group.toLowerCase().includes(needle) ||
		descriptor.description.toLowerCase().includes(needle)
	);
}
