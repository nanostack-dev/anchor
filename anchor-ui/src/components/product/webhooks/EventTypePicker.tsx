import type { WebhookEventTypeDescriptor } from "@/client";
import { listWebhookEventTypesOptions } from "@/client/@tanstack/react-query.gen";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { useQuery } from "@tanstack/react-query";
import { Search } from "lucide-react";
import { type ReactNode, useId, useMemo, useState } from "react";
import {
	type EventTypeGroup,
	type GroupSelection,
	allSubscriptions,
	coveredEventTypeCount,
	groupEventTypes,
	groupSelection,
	isEventTypeCovered,
	matchesEventTypeQuery,
	toggleEventType,
	toggleGroup,
} from "./webhook-display";

interface EventTypePickerProps {
	value: string[];
	onChange: (eventTypes: string[]) => void;
	disabled?: boolean;
}

/**
 * Subscription selector sourced from the registry catalog
 * (`GET /v1/webhook-event-types`): a filterable, always-visible checkbox tree
 * grouped by the part of the type before the dot.
 *
 * A group whose types follow the `<group>.<event>` grammar collapses to its
 * `<group>.*` wildcard when fully selected — shorter to store, and the only way
 * to stay subscribed to types added to that group later. The reverse holds too:
 * an endpoint saved with a wildcard draws its group fully checked, so the form
 * round-trips what it was given.
 */
export function EventTypePicker({
	value,
	onChange,
	disabled = false,
}: EventTypePickerProps) {
	const filterId = useId();
	const allEventsId = useId();
	const [query, setQuery] = useState("");
	const { data, isLoading, error } = useQuery(listWebhookEventTypesOptions());

	const descriptors = useMemo(() => data?.items ?? [], [data?.items]);
	const groups = useMemo(() => groupEventTypes(descriptors), [descriptors]);

	const selectedCount = coveredEventTypeCount(value, descriptors);
	const allSelected =
		descriptors.length > 0 && selectedCount === descriptors.length;

	// Filtering narrows what is *shown*. Selection state and the group toggle
	// keep operating on the whole group, so a filtered view can never silently
	// unsubscribe the rows it is hiding.
	const visibleGroups = useMemo(
		() =>
			groups
				.map((group) => ({
					group,
					visible: group.descriptors.filter((descriptor) =>
						matchesEventTypeQuery(descriptor, query),
					),
				}))
				.filter((entry) => entry.visible.length > 0),
		[groups, query],
	);

	if (error) {
		return (
			<p className="text-sm text-destructive">
				Failed to load the event type registry.
			</p>
		);
	}

	return (
		<div className="flex flex-col gap-3">
			<div className="flex items-center justify-between gap-4">
				<div className="flex flex-col">
					<Label htmlFor={allEventsId} className="text-sm font-medium">
						Subscribe to all events
					</Label>
					<span className="text-xs text-muted-foreground">
						Every group, including event types added later.
					</span>
				</div>
				<Switch
					id={allEventsId}
					checked={allSelected}
					disabled={disabled || isLoading || descriptors.length === 0}
					onCheckedChange={() =>
						onChange(allSelected ? [] : allSubscriptions(groups))
					}
				/>
			</div>

			<div className="relative">
				<Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
				<Input
					id={filterId}
					value={query}
					onChange={(event) => setQuery(event.target.value)}
					placeholder="Filter events..."
					className="pl-8"
					disabled={disabled || isLoading}
					aria-label="Filter event types"
				/>
			</div>

			<div className="max-h-64 overflow-y-auto rounded-md border p-2">
				{isLoading ? (
					<div className="flex flex-col gap-2">
						{["a", "b", "c", "d"].map((key) => (
							<Skeleton key={`event-type-skeleton-${key}`} className="h-6" />
						))}
					</div>
				) : visibleGroups.length === 0 ? (
					<p className="p-2 text-sm text-muted-foreground">
						{descriptors.length === 0
							? "No event types are registered."
							: `No event type matches “${query}”.`}
					</p>
				) : (
					<div className="flex flex-col gap-3">
						{visibleGroups.map(({ group, visible }) => (
							<GroupSection
								key={group.group}
								group={group}
								visible={visible}
								selection={groupSelection(value, group)}
								disabled={disabled}
								isCovered={(descriptor) =>
									isEventTypeCovered(value, descriptor)
								}
								onToggleGroup={() => onChange(toggleGroup(value, group))}
								onToggleType={(descriptor) =>
									onChange(toggleEventType(value, group, descriptor))
								}
							/>
						))}
					</div>
				)}
			</div>

			<div className="flex items-center justify-between gap-2">
				<p className="text-xs text-muted-foreground">
					{selectedCount === 0
						? "No events selected — this endpoint would receive nothing."
						: `${selectedCount} of ${descriptors.length} events selected.`}
					{data?.api_version ? (
						<>
							{" Envelope contract version "}
							<span className="font-mono">{data.api_version}</span>.
						</>
					) : null}
				</p>
				{value.length > 0 && (
					<Button
						type="button"
						variant="ghost"
						size="sm"
						className="h-auto shrink-0 px-2 py-1 text-xs text-muted-foreground hover:text-foreground"
						disabled={disabled}
						onClick={() => onChange([])}
					>
						Clear
					</Button>
				)}
			</div>
		</div>
	);
}

function GroupSection({
	group,
	visible,
	selection,
	disabled,
	isCovered,
	onToggleGroup,
	onToggleType,
}: {
	group: EventTypeGroup;
	visible: WebhookEventTypeDescriptor[];
	selection: GroupSelection;
	disabled: boolean;
	isCovered: (descriptor: WebhookEventTypeDescriptor) => boolean;
	onToggleGroup: () => void;
	onToggleType: (descriptor: WebhookEventTypeDescriptor) => void;
}) {
	return (
		<div className="flex flex-col gap-0.5">
			<PickerRow
				checked={selection === "all"}
				indeterminate={selection === "partial"}
				disabled={disabled}
				onToggle={onToggleGroup}
				label={
					<span className="text-sm font-medium">
						{group.group}
						{group.wildcard && (
							<span className="ml-2 font-mono text-xs font-normal text-muted-foreground">
								{group.wildcard}
							</span>
						)}
					</span>
				}
			/>
			<div className="flex flex-col gap-0.5 pl-6">
				{visible.map((descriptor) => (
					<PickerRow
						key={descriptor.type}
						checked={isCovered(descriptor)}
						disabled={disabled}
						onToggle={() => onToggleType(descriptor)}
						label={<span className="font-mono text-sm">{descriptor.type}</span>}
						description={descriptor.description}
					/>
				))}
			</div>
		</div>
	);
}

function PickerRow({
	checked,
	indeterminate = false,
	disabled,
	onToggle,
	label,
	description,
}: {
	checked: boolean;
	indeterminate?: boolean;
	disabled: boolean;
	onToggle: () => void;
	label: ReactNode;
	description?: string;
}) {
	return (
		<button
			type="button"
			disabled={disabled}
			onClick={onToggle}
			className={cn(
				"flex w-full items-start gap-2 rounded-md px-1 py-1 text-left transition-colors hover:bg-muted",
				disabled && "cursor-not-allowed opacity-60 hover:bg-transparent",
			)}
		>
			<Checkbox
				checked={checked}
				indeterminate={indeterminate}
				disabled={disabled}
				tabIndex={-1}
				className={cn(
					"pointer-events-none mt-0.5",
					// The shared checkbox draws a tick for checked and indeterminate
					// alike. Swap the glyph here rather than forking the primitive.
					indeterminate &&
						"border-primary bg-primary text-primary-foreground before:absolute before:inset-0 before:m-auto before:h-0.5 before:w-2 before:rounded-full before:bg-current [&_svg]:hidden",
				)}
			/>
			<span className="flex min-w-0 flex-col">
				{label}
				{description && (
					<span className="text-xs text-muted-foreground">{description}</span>
				)}
			</span>
		</button>
	);
}
