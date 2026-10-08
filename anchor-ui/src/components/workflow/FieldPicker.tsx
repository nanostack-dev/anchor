import type { WorkflowFieldType, WorkflowParamType } from "@/client";
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
import { type ReactElement, useState } from "react";
import { fieldTypeIcons } from "./field-icons";
import {
	expectedTypeLabel,
	fieldPathPattern,
	fieldTypeLabels,
	fits,
	isInsideJsonField,
} from "./fields";
import { type WorkflowVariable, groupBy } from "./workflow-model";

const optionValue = (field: WorkflowVariable) =>
	`${field.source} ${field.label} ${field.path}`;

function FieldOption({
	field,
	checked,
	onSelect,
}: {
	field: WorkflowVariable;
	checked: boolean;
	onSelect: () => void;
}) {
	const Icon = fieldTypeIcons[field.type];
	return (
		<CommandItem
			value={optionValue(field)}
			checked={checked}
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
 * A searchable list of the fields a value can use. Fields that fit a
 * parameter, or share a type, come first. With `allowPath`, a typed path
 * inside a listed JSON field (a metadata key) can be used as is.
 */
export function FieldPicker({
	fields,
	paramType,
	fitType,
	allowPath = false,
	selected,
	label,
	trigger,
	onPick,
}: {
	fields: WorkflowVariable[];
	paramType?: WorkflowParamType;
	fitType?: WorkflowFieldType;
	allowPath?: boolean;
	selected?: string;
	label: string;
	trigger?: ReactElement;
	onPick: (path: string) => void;
}) {
	const [open, setOpen] = useState(false);
	const [query, setQuery] = useState("");
	const expected = paramType
		? expectedTypeLabel(paramType)
		: fitType
			? fieldTypeLabels[fitType]
			: undefined;
	const fitting = paramType
		? expected
			? fields.filter((field) => fits(paramType, field.type))
			: []
		: fitType
			? fields.filter((field) => field.type === fitType)
			: [];
	const others = fields.filter((field) => !fitting.includes(field));
	const path = query.trim();
	const offerPath =
		allowPath &&
		fieldPathPattern.test(path) &&
		!fields.some((field) => field.path === path) &&
		isInsideJsonField(fields, path);
	const current = fields.find((field) => field.path === selected);
	const changeOpen = (next: boolean) => {
		setOpen(next);
		if (!next) setQuery("");
	};
	const pick = (picked: string) => {
		onPick(picked);
		changeOpen(false);
	};
	const option = (field: WorkflowVariable) => (
		<FieldOption
			key={field.path}
			field={field}
			checked={field.path === selected}
			onSelect={() => pick(field.path)}
		/>
	);

	return (
		<Popover open={open} onOpenChange={changeOpen}>
			<PopoverTrigger
				render={
					trigger ?? (
						<Button
							variant="ghost"
							size="xs"
							icon={Braces}
							aria-label={label}
							disabled={fields.length === 0}
						/>
					)
				}
			/>
			<PopoverContent align={trigger ? "start" : "end"} aria-label={label}>
				<Command defaultValue={current ? optionValue(current) : undefined}>
					<CommandInput
						placeholder={allowPath ? "Search or type a path" : "Search fields"}
						aria-label="Search fields"
						value={query}
						onValueChange={setQuery}
						autoFocus
					/>
					<CommandList>
						{offerPath ? (
							<CommandGroup heading="Path">
								<CommandItem value={`path ${path}`} onSelect={() => pick(path)}>
									<Braces aria-hidden />
									<Box
										as="span"
										className="min-w-0 flex-1 truncate font-mono text-xs"
									>
										Use {path}
									</Box>
								</CommandItem>
							</CommandGroup>
						) : null}
						<CommandEmpty>
							{allowPath
								? "No field matches. A key inside a JSON field can be typed as a path, such as steps.org.metadata.plan."
								: "No field matches."}
						</CommandEmpty>
						{fitting.length > 0 ? (
							<CommandGroup
								heading={
									paramType ? `Fits: ${expected}` : `Same type: ${expected}`
								}
							>
								{fitting.map(option)}
							</CommandGroup>
						) : null}
						{groupBy(others, (field) => field.source).map(([source, group]) => (
							<CommandGroup key={source} heading={source}>
								{group.map(option)}
							</CommandGroup>
						))}
					</CommandList>
				</Command>
			</PopoverContent>
		</Popover>
	);
}
