import { Badge } from "@/components/ui/badge";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";

interface EventTypeChipsProps {
	eventTypes: string[];
	/** How many chips to render before collapsing the rest into `+N`. */
	max?: number;
}

/**
 * Compact, table-friendly rendering of an endpoint's subscriptions: the first
 * few types as chips, the remainder behind a `+N` chip whose tooltip lists
 * them in full.
 */
export function EventTypeChips({ eventTypes, max = 3 }: EventTypeChipsProps) {
	if (eventTypes.length === 0) {
		return <span className="text-sm text-muted-foreground">—</span>;
	}

	const visible = eventTypes.slice(0, max);
	const overflow = eventTypes.slice(max);

	return (
		<div className="flex flex-wrap items-center gap-1">
			{visible.map((eventType) => (
				<Badge key={eventType} variant="outline" className="font-mono text-xs">
					{eventType}
				</Badge>
			))}
			{overflow.length > 0 && (
				<Tooltip>
					<TooltipTrigger
						render={
							<Badge variant="secondary" className="text-xs">
								{`+${overflow.length}`}
							</Badge>
						}
					/>
					<TooltipContent>
						<span className="font-mono">{overflow.join(", ")}</span>
					</TooltipContent>
				</Tooltip>
			)}
		</div>
	);
}
