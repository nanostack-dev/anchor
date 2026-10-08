import {
	type WorkflowCatalogResponse,
	WorkflowParamType,
	type WorkflowStep,
} from "@/client";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { IconButton } from "@nanostackorg/design-system/components/button";
import {
	Collapsible,
	CollapsibleContent,
	CollapsibleTrigger,
} from "@nanostackorg/design-system/components/collapsible";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import { Switch } from "@nanostackorg/design-system/components/switch";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	ArrowDown,
	ArrowLeft,
	ArrowRight,
	ArrowUp,
	ChevronRight,
	CircleAlert,
	Filter,
	Repeat,
	Trash2,
	X,
	Zap,
} from "lucide-react";
import { useId } from "react";
import { ConditionEditor } from "./ConditionEditor";
import { DataPanel } from "./DataPanel";
import { EventDataEditor } from "./EventDataEditor";
import { ParamInput } from "./ParamInput";
import { actionGroupIcon } from "./action-icons";
import { fieldTypeIcons } from "./field-icons";
import { FieldTargetProvider } from "./field-target";
import { fieldTypeLabels } from "./fields";
import type { WorkflowResources } from "./useWorkflowResources";
import { type WorkflowVariable, findAction } from "./workflow-model";

export interface StepStart {
	event: string;
	workflowNames: string[];
}

export function StepEditor({
	step,
	index,
	count,
	catalog,
	variables,
	resources,
	errors = {},
	starts = [],
	loopWarning,
	onChange,
	onMove,
	onRemove,
	onClose,
	closeLabel = "Close",
}: {
	step: WorkflowStep;
	index: number;
	count: number;
	catalog: WorkflowCatalogResponse;
	variables: WorkflowVariable[];
	resources: WorkflowResources;
	errors?: Record<string, string>;
	starts?: StepStart[];
	loopWarning?: string;
	onChange: (step: WorkflowStep) => void;
	onMove: (to: number) => void;
	onRemove: () => void;
	onClose?: () => void;
	closeLabel?: string;
}) {
	const continueId = useId();
	const action = findAction(catalog, step.action);
	const ActionIcon = actionGroupIcon(action?.group);
	const when = step.when ?? [];
	const setParam = (name: string, value: string) =>
		onChange({ ...step, params: { ...step.params, [name]: value } });
	const typesParam = action?.params.find(
		(param) => param.type === WorkflowParamType.FIELD_TYPES && param.types,
	);
	const typedParam = typesParam
		? action?.params.find((param) => param.name === typesParam.types)
		: undefined;
	const plainParams = (action?.params ?? []).filter(
		(param) => param !== typesParam && param !== typedParam,
	);
	const readsFields = plainParams.some(
		(param) => !param.literal && !param.options?.length,
	);

	return (
		<Box
			as="article"
			aria-label={`Step ${index + 1}: ${action?.name ?? step.action}`}
			className="relative"
		>
			<Box className="flex items-start gap-3 border-b border-border px-4 py-3">
				<Box className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
					<ActionIcon className="size-4" aria-hidden />
				</Box>
				<Box className="min-w-0 flex-1 space-y-1">
					<Inline space="xs" alignY="center" wrap>
						<Text as="span" size="xs" tone="muted" tabular>
							Step {index + 1}
						</Text>
						<Text as="span" size="sm" weight="semibold">
							{action?.name ?? step.action}
						</Text>
						{action && !action.writes ? (
							<Badge variant="outline" tone="neutral">
								Read only
							</Badge>
						) : null}
					</Inline>
					<Input
						size="sm"
						variant="ghost"
						aria-label={`Name of step ${index + 1}`}
						placeholder="Describe what this step is for"
						value={step.name ?? ""}
						onChange={(event) =>
							onChange({ ...step, name: event.target.value })
						}
					/>
				</Box>
				<Inline space="xxs">
					<IconButton
						variant="ghost"
						size="sm"
						icon={ArrowUp}
						label="Move step up"
						disabled={index === 0}
						onClick={() => onMove(index - 1)}
					/>
					<IconButton
						variant="ghost"
						size="sm"
						icon={ArrowDown}
						label="Move step down"
						disabled={index === count - 1}
						onClick={() => onMove(index + 1)}
					/>
					<IconButton
						variant="ghost"
						size="sm"
						tone="critical"
						icon={Trash2}
						label="Remove step"
						onClick={onRemove}
					/>
					{onClose ? (
						<IconButton
							variant="ghost"
							size="sm"
							icon={closeLabel === "Back to flow" ? ArrowLeft : X}
							label={closeLabel}
							onClick={onClose}
						/>
					) : null}
				</Inline>
			</Box>

			<FieldTargetProvider>
				<Stack space="lg">
					<Box className="space-y-4 px-4 py-4">
						{loopWarning ? (
							<Alert tone="warning" icon={Repeat}>
								<AlertTitle>This step would start a loop</AlertTitle>
								<AlertDescription>
									<Box className="[overflow-wrap:anywhere]">{loopWarning}</Box>
								</AlertDescription>
							</Alert>
						) : null}
						{errors[""] ? (
							<Alert tone="critical" icon={CircleAlert}>
								<AlertDescription>{errors[""]}</AlertDescription>
							</Alert>
						) : null}
						{action ? (
							<Text size="sm" tone="muted">
								{action.description}
							</Text>
						) : (
							<Text size="sm" tone="critical">
								This action is not in the catalog any more.
							</Text>
						)}
						{readsFields || typesParam ? (
							<DataPanel fields={variables} />
						) : null}
						<Box className="grid grid-cols-1 gap-4 md:grid-cols-2">
							{plainParams.map((param) => (
								<Box
									key={param.name}
									className={
										param.type === "json" ? "md:col-span-2" : undefined
									}
								>
									<ParamInput
										param={param}
										value={step.params[param.name] ?? ""}
										variables={variables}
										resources={resources}
										error={errors[param.name]}
										onChange={(value) => setParam(param.name, value)}
									/>
								</Box>
							))}
							{typesParam && typedParam ? (
								<Box className="md:col-span-2">
									<EventDataEditor
										dataParam={typedParam}
										typesParam={typesParam}
										data={step.params[typedParam.name] ?? ""}
										types={step.params[typesParam.name] ?? ""}
										variables={variables}
										errors={errors}
										onChange={(next) =>
											onChange({
												...step,
												params: {
													...step.params,
													[typedParam.name]: next.data,
													[typesParam.name]: next.types,
												},
											})
										}
									/>
								</Box>
							) : null}
						</Box>

						<Collapsible defaultOpen={when.length > 0}>
							<CollapsibleTrigger
								render={
									<button
										type="button"
										className="group flex items-center gap-2 text-sm font-medium text-muted-foreground hover:text-foreground"
									/>
								}
							>
								<ChevronRight className="size-4 transition-transform group-data-[panel-open]:rotate-90" />
								<Filter className="size-3.5" aria-hidden />
								Only run this step when…
								{when.length > 0 ? (
									<Badge variant="soft" tone="brand">
										{when.length}
									</Badge>
								) : null}
							</CollapsibleTrigger>
							<CollapsibleContent>
								<Box className="mt-3 rounded-lg bg-muted/50 p-3">
									<ConditionEditor
										label={`Step ${index + 1} condition`}
										conditions={when}
										variables={variables}
										onChange={(next) => onChange({ ...step, when: next })}
										emptyLabel="The step always runs."
									/>
								</Box>
							</CollapsibleContent>
						</Collapsible>

						{starts.length > 0 ? (
							<Box
								as="ul"
								aria-label={`Events step ${index + 1} emits`}
								className="space-y-1 rounded-lg border border-dashed border-border px-3 py-2"
							>
								{starts.map((start) => (
									<Box
										as="li"
										key={start.event}
										className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground"
									>
										<Zap className="size-3 shrink-0" aria-hidden />
										<Box as="span">Emits</Box>
										<Box
											as="span"
											className="break-all font-mono text-foreground"
										>
											{start.event}
										</Box>
										{start.workflowNames.length > 0 ? (
											<>
												<ArrowRight className="size-3 shrink-0" aria-hidden />
												<Box as="span" className="break-words">
													starts{" "}
													{start.workflowNames
														.map((name) => `“${name}”`)
														.join(", ")}
												</Box>
											</>
										) : null}
									</Box>
								))}
							</Box>
						) : null}

						<Box className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-3">
							<Inline space="sm" alignY="center">
								<Switch
									id={continueId}
									checked={step.continue_on_error ?? false}
									onCheckedChange={(checked) =>
										onChange({ ...step, continue_on_error: checked })
									}
								/>
								<Label htmlFor={continueId} size="sm">
									Keep going if this step fails
								</Label>
							</Inline>
							{action && action.outputs.length > 0 ? (
								<Inline space="xs" alignY="center" wrap>
									<Text as="span" size="xs" tone="muted">
										Gives later steps
									</Text>
									{action.outputs.map((output) => {
										const OutputIcon = fieldTypeIcons[output.type];
										return (
											<Box
												as="span"
												key={output.name}
												title={`${fieldTypeLabels[output.type]}: ${output.description}`}
												className="inline-flex items-center gap-1 break-all rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-muted-foreground"
											>
												<OutputIcon className="size-3 shrink-0" aria-hidden />
												steps.{step.id}.{output.name}
											</Box>
										);
									})}
								</Inline>
							) : null}
						</Box>
					</Box>
				</Stack>
			</FieldTargetProvider>
		</Box>
	);
}
