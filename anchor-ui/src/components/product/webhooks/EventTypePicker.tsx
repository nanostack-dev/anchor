import { listWebhookEventTypesOptions } from "@/client/@tanstack/react-query.gen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { cn } from "@/lib/utils";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, X } from "lucide-react";
import { type ReactNode, useMemo } from "react";
import { groupEventTypes, isEventTypeCovered } from "./webhook-display";

interface EventTypePickerProps {
	value: string[];
	onChange: (eventTypes: string[]) => void;
	disabled?: boolean;
}

/**
 * Event-type picker sourced from the registry catalog
 * (`GET /v1/webhook-event-types`), grouped by event group. A group with the
 * `<group>.<event>` grammar can be subscribed wholesale as `<group>.*`, which
 * also picks up event types added to that group later.
 */
export function EventTypePicker({
	value,
	onChange,
	disabled = false,
}: EventTypePickerProps) {
	const { data, isLoading, error } = useQuery(listWebhookEventTypesOptions());

	const groups = useMemo(
		() => groupEventTypes(data?.items ?? []),
		[data?.items],
	);

	const toggleWildcard = (wildcard: string, group: string) => {
		if (value.includes(wildcard)) {
			onChange(value.filter((entry) => entry !== wildcard));
			return;
		}
		// The wildcard subsumes every exact type of the group; keeping both
		// would send redundant subscriptions the server would have to dedupe.
		const groupPrefix = `${group}.`;
		onChange([
			...value.filter(
				(entry) => entry !== wildcard && !entry.startsWith(groupPrefix),
			),
			wildcard,
		]);
	};

	const toggleType = (eventType: string) => {
		onChange(
			value.includes(eventType)
				? value.filter((entry) => entry !== eventType)
				: [...value, eventType],
		);
	};

	const summary =
		value.length === 0
			? "Select event types"
			: `${value.length} subscription${value.length === 1 ? "" : "s"} selected`;

	return (
		<div className="flex flex-col gap-2">
			<Popover>
				<PopoverTrigger
					render={
						<Button
							type="button"
							variant="outline"
							className="w-full justify-between"
							disabled={disabled || isLoading}
						/>
					}
				>
					<span className={value.length === 0 ? "text-muted-foreground" : ""}>
						{summary}
					</span>
					{isLoading ? (
						<Spinner className="text-muted-foreground" />
					) : (
						<ChevronDown className="size-4 text-muted-foreground" />
					)}
				</PopoverTrigger>
				<PopoverContent
					className="max-h-80 w-96 overflow-y-auto p-2"
					align="start"
				>
					{error ? (
						<p className="p-2 text-sm text-destructive">
							Failed to load the event type registry.
						</p>
					) : groups.length === 0 ? (
						<p className="p-2 text-sm text-muted-foreground">
							No event types are registered.
						</p>
					) : (
						<div className="flex flex-col gap-3">
							{groups.map((group) => {
								const wildcardSelected =
									group.wildcard !== null && value.includes(group.wildcard);
								return (
									<div key={group.group} className="flex flex-col gap-1">
										<p className="px-1 text-xs font-medium text-muted-foreground">
											{group.group}
										</p>
										{group.wildcard && (
											<PickerRow
												checked={wildcardSelected}
												disabled={disabled}
												onToggle={() =>
													group.wildcard &&
													toggleWildcard(group.wildcard, group.group)
												}
												label={
													<span className="font-mono text-sm">
														{group.wildcard}
													</span>
												}
												description={`Every ${group.group} event, including types added later.`}
											/>
										)}
										{group.descriptors.map((descriptor) => (
											<PickerRow
												key={descriptor.type}
												checked={isEventTypeCovered(value, descriptor)}
												disabled={disabled || wildcardSelected}
												onToggle={() => toggleType(descriptor.type)}
												label={
													<span className="font-mono text-sm">
														{descriptor.type}
													</span>
												}
												description={
													wildcardSelected
														? `Covered by ${group.wildcard}`
														: descriptor.description
												}
											/>
										))}
									</div>
								);
							})}
						</div>
					)}
				</PopoverContent>
			</Popover>

			{value.length > 0 && (
				<div className="flex flex-wrap gap-1">
					{value.map((eventType) => (
						<Badge
							key={eventType}
							variant="secondary"
							className="gap-1 font-mono text-xs"
						>
							{eventType}
							<button
								type="button"
								aria-label={`Remove ${eventType}`}
								disabled={disabled}
								onClick={() =>
									onChange(value.filter((entry) => entry !== eventType))
								}
							>
								<X className="size-3" />
							</button>
						</Badge>
					))}
				</div>
			)}
			{data?.api_version && (
				<p className="text-xs text-muted-foreground">
					Envelope contract version{" "}
					<span className="font-mono">{data.api_version}</span>.
				</p>
			)}
		</div>
	);
}

function PickerRow({
	checked,
	disabled,
	onToggle,
	label,
	description,
}: {
	checked: boolean;
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
				"flex w-full items-start gap-2 rounded-md px-1 py-1.5 text-left transition-colors hover:bg-muted",
				disabled && "cursor-not-allowed opacity-60 hover:bg-transparent",
			)}
		>
			<Checkbox
				checked={checked}
				disabled={disabled}
				tabIndex={-1}
				className="pointer-events-none mt-0.5"
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
