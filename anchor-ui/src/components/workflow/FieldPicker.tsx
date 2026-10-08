import type { WorkflowParamType } from "@/client";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Command,
	CommandEmpty,
	CommandGroup,
	CommandInput,
	CommandItem,
	CommandList,
} from "@nanostackorg/design-system/components/command";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@nanostackorg/design-system/components/popover";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Braces } from "lucide-react";
import { useState } from "react";
import { fieldTypeIcons } from "./field-icons";
import { expectedTypeLabel, fieldTypeLabels, fits } from "./fields";
import { type WorkflowVariable, groupBy } from "./workflow-model";

function FieldOption({
	field,
	onSelect,
}: {
	field: WorkflowVariable;
	onSelect: () => void;
}) {
	const Icon = fieldTypeIcons[field.type];
	return (
		<CommandItem
			value={`${field.source} ${field.label} ${field.path}`}
			onSelect={onSelect}
		>
			<Icon aria-hidden />
			<Box as="span" className="flex min-w-0 flex-1 flex-col">
				<Box as="span" className="truncate font-mono text-xs">
					{field.label}
				</Box>
				<Box as="span" className="truncate text-[11px] text-muted-foreground">
					{fieldTypeLabels[field.type]}
				</Box>
			</Box>
		</CommandItem>
	);
}

/**
 * A searchable list of the fields a value can use. For a parameter that
 * expects one kind of value, the fields that fit come first.
 */
export function FieldPicker({
	fields,
	paramType,
	label,
	onPick,
}: {
	fields: WorkflowVariable[];
	paramType?: WorkflowParamType;
	label: string;
	onPick: (path: string) => void;
}) {
	const [open, setOpen] = useState(false);
	const expected = paramType ? expectedTypeLabel(paramType) : undefined;
	const fitting =
		paramType && expected
			? fields.filter((field) => fits(paramType, field.type))
			: [];
	const others = fields.filter((field) => !fitting.includes(field));
	const pick = (path: string) => {
		onPick(path);
		setOpen(false);
	};

	return (
		<Popover open={open} onOpenChange={setOpen}>
			<PopoverTrigger
				render={
					<Button
						variant="ghost"
						size="xs"
						icon={Braces}
						aria-label={label}
						disabled={fields.length === 0}
					/>
				}
			/>
			<PopoverContent align="end" aria-label={label}>
				<Command>
					<CommandInput
						placeholder="Search fields"
						aria-label="Search fields"
						autoFocus
					/>
					<CommandList>
						<CommandEmpty>No field matches.</CommandEmpty>
						{fitting.length > 0 ? (
							<CommandGroup heading={`Fits: ${expected}`}>
								{fitting.map((field) => (
									<FieldOption
										key={field.path}
										field={field}
										onSelect={() => pick(field.path)}
									/>
								))}
							</CommandGroup>
						) : null}
						{groupBy(others, (field) => field.source).map(([source, group]) => (
							<CommandGroup key={source} heading={source}>
								{group.map((field) => (
									<FieldOption
										key={field.path}
										field={field}
										onSelect={() => pick(field.path)}
									/>
								))}
							</CommandGroup>
						))}
					</CommandList>
				</Command>
			</PopoverContent>
		</Popover>
	);
}
