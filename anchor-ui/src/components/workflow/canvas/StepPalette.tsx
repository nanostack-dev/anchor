import type { WorkflowActionResponse } from "@/client";
import { cn } from "@/lib/utils";
import { Button } from "@nanostackorg/design-system/components/button";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { GripVertical, ListPlus } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { useId } from "react";
import { actionGroupIcon, actionGroupTone } from "../action-icons";
import { groupBy } from "../workflow-model";
import { ACTION_DRAG_TYPE } from "./canvas-context";
import { EASE_OUT_CLASS, easeOut } from "./motion";

/**
 * `panel` floats beside the workflow on a wide canvas; `tray` sits along the
 * bottom of a narrow one, where a side panel would cover the column.
 */
export type StepPalettePlacement = "panel" | "tray";

export const PALETTE_PANEL_WIDTH = 220;

interface StepPaletteProps {
	actions: WorkflowActionResponse[];
	open: boolean;
	onOpenChange: (open: boolean) => void;
	placement: StepPalettePlacement;
	/** Appends the step at the end of the workflow. */
	onAdd: (action: WorkflowActionResponse) => void;
	onDragChange: (dragging: boolean) => void;
	/** A step from the palette is being dragged. */
	dragging?: boolean;
}

function PaletteItem({
	action,
	indented,
	onAdd,
	onDragChange,
}: Pick<StepPaletteProps, "onAdd" | "onDragChange"> & {
	action: WorkflowActionResponse;
	indented: boolean;
}) {
	return (
		<button
			type="button"
			draggable
			aria-label={`Add step: ${action.name}`}
			title={action.description || undefined}
			onClick={() => onAdd(action)}
			onDragStart={(event) => {
				event.dataTransfer.setData(ACTION_DRAG_TYPE, action.type);
				event.dataTransfer.setData("text/plain", action.name);
				event.dataTransfer.effectAllowed = "copy";
				onDragChange(true);
			}}
			onDragEnd={() => onDragChange(false)}
			className={cn(
				"group flex w-full cursor-grab items-center gap-2 rounded-lg py-1.5 pr-1.5 text-left text-sm text-foreground outline-none",
				indented ? "pl-9" : "pl-2",
				"transition-[scale,background-color] duration-150 hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring/60 active:scale-[0.98] active:cursor-grabbing",
				"motion-reduce:transition-[opacity,background-color] motion-reduce:active:scale-100 motion-reduce:active:opacity-70",
				EASE_OUT_CLASS,
			)}
		>
			<Box
				as="span"
				className="line-clamp-2 min-w-0 flex-1 [overflow-wrap:anywhere]"
			>
				{action.name}
			</Box>
			<GripVertical
				className="size-3.5 shrink-0 text-muted-foreground opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-visible:opacity-100"
				aria-hidden
			/>
		</button>
	);
}

export function StepPalette({
	actions,
	open,
	onOpenChange,
	placement,
	onAdd,
	onDragChange,
	dragging = false,
}: StepPaletteProps) {
	const panelId = useId();
	const reduceMotion = useReducedMotion();
	const groups = groupBy(actions, (action) => action.group);
	const tray = placement === "tray";
	const steppingAside = dragging && tray;
	const from = reduceMotion
		? { opacity: 0 }
		: {
				opacity: 0,
				transform: tray
					? "translateY(8px) scale(0.98)"
					: "translateY(-4px) scale(0.98)",
			};
	const shown = reduceMotion
		? { opacity: steppingAside ? 0 : 1 }
		: {
				opacity: steppingAside ? 0 : 1,
				transform: steppingAside
					? "translateY(16px) scale(1)"
					: "translateY(0px) scale(1)",
			};

	return (
		<>
			<Box className="absolute top-3 left-3 z-20">
				<Button
					variant="outline"
					size="sm"
					icon={ListPlus}
					aria-expanded={open}
					aria-controls={panelId}
					onClick={() => onOpenChange(!open)}
				>
					Steps
				</Button>
			</Box>
			<AnimatePresence initial={false}>
				{open ? (
					<motion.section
						key="palette"
						id={panelId}
						aria-label="Steps to add"
						initial={from}
						animate={shown}
						exit={{ opacity: 0, transition: { duration: 0.1 } }}
						transition={{ duration: 0.18, ease: easeOut }}
						className={cn(
							"absolute z-20 flex flex-col overflow-hidden rounded-xl border border-border bg-card shadow-md",
							tray
								? "inset-x-3 bottom-3 max-h-[40%] origin-bottom"
								: "top-14 left-3 max-h-[calc(100%-11rem)] origin-top-left",
							steppingAside && "pointer-events-none",
						)}
						style={tray ? undefined : { width: PALETTE_PANEL_WIDTH }}
					>
						<Box className="border-b border-border px-3 py-2">
							<Text size="xs" tone="muted">
								Drag onto the flow, or click to add at the end.
							</Text>
						</Box>
						<Box className="min-h-0 flex-1 overflow-y-auto overscroll-contain p-1.5">
							{groups.length === 0 ? (
								<Box className="px-1.5 py-2">
									<Text size="sm" tone="muted">
										No steps are available to add.
									</Text>
								</Box>
							) : (
								groups.map(([group, items], index) => {
									const Icon = actionGroupIcon(group);
									const headingId = `${panelId}-group-${index}`;
									return (
										<Box key={group} className="pb-1">
											<Box
												id={headingId}
												className="flex items-center gap-2 px-2 pt-1.5 pb-1"
											>
												<Box
													as="span"
													className={cn(
														"flex size-5 shrink-0 items-center justify-center rounded-md",
														actionGroupTone(group),
													)}
												>
													<Icon className="size-3" aria-hidden />
												</Box>
												<Box
													as="span"
													title={group}
													className="min-w-0 truncate text-[11px] font-medium tracking-wide text-muted-foreground uppercase"
												>
													{group}
												</Box>
											</Box>
											<Box
												as="ul"
												aria-labelledby={headingId}
												className={cn(
													tray
														? "grid grid-cols-[repeat(auto-fill,minmax(9.5rem,1fr))] gap-x-1"
														: "flex flex-col",
												)}
											>
												{items.map((action) => (
													<Box as="li" key={action.type}>
														<PaletteItem
															action={action}
															indented={!tray}
															onAdd={onAdd}
															onDragChange={onDragChange}
														/>
													</Box>
												))}
											</Box>
										</Box>
									);
								})
							)}
						</Box>
					</motion.section>
				) : null}
			</AnimatePresence>
		</>
	);
}
