import { ScrollArea } from "@nanostackorg/design-system/components/scroll-area";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { FieldChip } from "./FieldChip";
import { useFieldTargets } from "./field-target";
import { type WorkflowVariable, groupBy } from "./workflow-model";

/**
 * Every field a step can read, as chips to drag onto an input, or to click
 * into the input used last.
 */
const SCROLLING_FIELDS = 8;

export function DataPanel({ fields }: { fields: WorkflowVariable[] }) {
	const targets = useFieldTargets();
	if (fields.length === 0) return null;
	const target = targets?.active;
	return (
		<Box
			as="section"
			aria-label="Data you can use"
			className="rounded-lg border border-dashed border-border bg-muted/30 p-3"
		>
			<Box className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
				<Text as="p" size="xs" weight="semibold">
					Data you can use
				</Text>
				<Box as="p" className="text-xs text-muted-foreground">
					{target
						? `Click to insert into ${target.label}, or drag onto any field.`
						: "Drag a field onto an input, or click an input first."}
				</Box>
			</Box>
			<Box
				className={
					fields.length > SCROLLING_FIELDS
						? "mt-2 flex h-36 flex-col"
						: "mt-2 flex flex-col"
				}
			>
				<ScrollArea height="fill" overscroll="contain">
					<Box className="space-y-2 pr-3">
						{groupBy(fields, (field) => field.source).map(([source, group]) => (
							<Box as="section" key={source} aria-label={`From ${source}`}>
								<Box
									as="p"
									className="text-[11px] font-medium text-muted-foreground"
								>
									{source}
								</Box>
								<Box className="mt-1 flex flex-wrap gap-1.5">
									{group.map((field) => (
										<FieldChip
											key={field.path}
											field={field}
											onPick={target?.insert}
										/>
									))}
								</Box>
							</Box>
						))}
					</Box>
				</ScrollArea>
			</Box>
		</Box>
	);
}
