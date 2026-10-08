import { Box } from "@nanostackorg/design-system/layout/box";
import { fieldTypeIcons } from "./field-icons";
import { FIELD_DRAG_TYPE, fieldTypeLabels } from "./fields";
import { type WorkflowVariable, reference } from "./workflow-model";

export function startFieldDrag(
	event: React.DragEvent,
	field: WorkflowVariable,
) {
	event.dataTransfer.setData(FIELD_DRAG_TYPE, field.path);
	event.dataTransfer.setData("text/plain", reference(field.path));
	event.dataTransfer.effectAllowed = "copy";
}

export function FieldChip({
	field,
	onPick,
}: {
	field: WorkflowVariable;
	onPick?: (path: string) => void;
}) {
	const Icon = fieldTypeIcons[field.type];
	return (
		<button
			type="button"
			draggable
			onDragStart={(event) => startFieldDrag(event, field)}
			onClick={() => onPick?.(field.path)}
			aria-label={`${field.label}, ${fieldTypeLabels[field.type]}, from ${field.source}`}
			title={
				field.description ? `${field.path}: ${field.description}` : field.path
			}
			className="inline-flex max-w-full cursor-grab items-center gap-1.5 rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground shadow-xs outline-none transition-[transform,border-color,box-shadow] duration-150 ease-out hover:border-primary/50 hover:shadow-sm focus-visible:ring-2 focus-visible:ring-ring/60 active:scale-[0.97] active:cursor-grabbing motion-reduce:active:scale-100"
		>
			<Icon className="size-3 shrink-0 text-muted-foreground" aria-hidden />
			<Box as="span" className="truncate font-mono">
				{field.label}
			</Box>
		</button>
	);
}
