import { Handle, type Node, type NodeProps, Position } from "@xyflow/react";
import { LockIcon, PlusIcon, RepeatIcon } from "lucide-react";
import {
	type KeyboardEvent,
	createContext,
	useContext,
	useEffect,
	useRef,
	useState,
} from "react";

import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@nanostackorg/design-system/components/tooltip";
import type { Direction } from "./resource-type-layout";
import { NODE_HEIGHT, NODE_WIDTH } from "./resource-type-layout";
import type { TreeType } from "./resource-type-tree";

export type ActionSource = "pointer" | "keyboard";

export interface CanvasActions {
	select(key: string | null): void;
	move(key: string, direction: Direction): void;
	addChild(key: string, source: ActionSource): void;
	startRename(key: string): void;
	commitName(key: string, name: string, source: ActionSource): void;
	cancelName(key: string): void;
	requestDelete(key: string, source: ActionSource): void;
}

export const CanvasActionsContext = createContext<CanvasActions | null>(null);

export type NodeTrace = "path" | "granted";

export interface ResourceTypeNodeData extends Record<string, unknown> {
	type: TreeType;
	trace?: NodeTrace;
	holders?: string[];
	editing: boolean;
	selected: boolean;
	naming: boolean;
	entering: boolean;
	tabbable: boolean;
	focusId?: number;
	addBlocker?: string;
}

export type ResourceTypeFlowNode = Node<ResourceTypeNodeData, "resourceType">;

const ARROWS: Record<string, Direction> = {
	ArrowUp: "up",
	ArrowDown: "down",
	ArrowLeft: "left",
	ArrowRight: "right",
};

function NameInput({
	type,
	actions,
}: {
	type: TreeType;
	actions: CanvasActions;
}) {
	const [value, setValue] = useState(type.name);
	const settled = useRef(false);
	const input = useRef<HTMLInputElement>(null);
	useEffect(() => {
		input.current?.focus();
		input.current?.select();
	}, []);
	const settle = (wantsCommit: boolean, source: ActionSource) => {
		if (settled.current) return;
		const empty = !value.trim();
		if (wantsCommit && empty && source === "keyboard") return;
		settled.current = true;
		if (wantsCommit && !empty)
			actions.commitName(type.key, value.trim(), source);
		else actions.cancelName(type.key);
	};
	return (
		<input
			ref={input}
			aria-label={type.published ? `Rename ${type.name}` : "New type name"}
			placeholder="Name, e.g. Folder"
			value={value}
			maxLength={120}
			onChange={(event) => setValue(event.target.value)}
			onKeyDown={(event) => {
				event.stopPropagation();
				if (event.key === "Enter") {
					event.preventDefault();
					settle(true, "keyboard");
				}
				if (event.key === "Escape") {
					event.preventDefault();
					settle(false, "keyboard");
				}
			}}
			onBlur={() => settle(true, "pointer")}
			className="nodrag nopan h-7 w-full min-w-0 rounded-md border border-input bg-background px-2 text-sm text-foreground outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/30"
		/>
	);
}

export function ResourceTypeNode({ data }: NodeProps<ResourceTypeFlowNode>) {
	const actions = useContext(CanvasActionsContext);
	if (!actions) throw new Error("ResourceTypeNode needs CanvasActionsContext");
	const {
		type,
		editing,
		selected,
		naming,
		entering,
		tabbable,
		focusId,
		addBlocker,
		trace,
		holders = [],
	} = data;
	const label = type.name || "New type";
	const card = useRef<HTMLDivElement>(null);
	const focused = useRef<number | undefined>(undefined);
	useEffect(() => {
		if (focusId === undefined || naming || focused.current === focusId) return;
		focused.current = focusId;
		card.current?.focus();
	}, [focusId, naming]);

	const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
		if (event.target !== event.currentTarget) return;
		const direction = ARROWS[event.key];
		if (direction) {
			event.preventDefault();
			actions.move(type.key, direction);
			return;
		}
		if (!editing) return;
		if (event.key === "Enter" && !type.builtIn) {
			event.preventDefault();
			actions.startRename(type.key);
		} else if (event.key === "n" || event.key === "N") {
			event.preventDefault();
			actions.addChild(type.key, "keyboard");
		} else if (event.key === "Delete" || event.key === "Backspace") {
			event.preventDefault();
			actions.requestDelete(type.key, "keyboard");
		}
	};

	return (
		<div
			className="resource-type-node group relative"
			style={{ width: NODE_WIDTH, height: NODE_HEIGHT }}
		>
			<Handle
				type="target"
				position={Position.Top}
				isConnectable={false}
				className="!pointer-events-none !opacity-0"
			/>
			<div
				ref={card}
				role={naming ? undefined : "button"}
				tabIndex={naming ? undefined : tabbable ? 0 : -1}
				aria-label={naming ? undefined : label}
				aria-pressed={naming ? undefined : selected}
				aria-roledescription={naming ? undefined : "resource type"}
				aria-describedby={naming ? undefined : `${type.key}-description`}
				data-rt-key={type.key}
				data-selected={selected}
				data-entering={entering}
				data-trace={trace}
				onKeyDown={naming ? undefined : onKeyDown}
				onFocus={(event) => {
					if (event.target === event.currentTarget) actions.select(type.key);
				}}
				onDoubleClick={() => {
					if (editing && !type.builtIn) actions.startRename(type.key);
				}}
				className="resource-type-card flex h-full w-full cursor-pointer flex-col justify-center gap-1 rounded-xl border border-border bg-card px-3 shadow-xs outline-none select-none focus-visible:ring-3 focus-visible:ring-ring/30 data-[selected=true]:border-primary data-[selected=true]:ring-2 data-[selected=true]:ring-primary/20 data-[trace=path]:border-primary data-[trace=path]:bg-accent data-[trace=granted]:border-success data-[trace=granted]:ring-2 data-[trace=granted]:ring-success/25"
			>
				<div className="flex min-w-0 items-center gap-1.5">
					{type.builtIn && (
						<Tooltip>
							<TooltipTrigger
								render={
									<span
										role="img"
										aria-label="Built in"
										className="nodrag nopan inline-flex shrink-0 text-muted-foreground"
									/>
								}
							>
								<LockIcon aria-hidden className="size-3.5" />
							</TooltipTrigger>
							<TooltipContent>Built in</TooltipContent>
						</Tooltip>
					)}
					{naming ? (
						<NameInput type={type} actions={actions} />
					) : (
						<span
							className="min-w-0 truncate font-medium text-foreground text-sm"
							title={label}
						>
							{label}
						</span>
					)}
				</div>
				<div className="flex min-w-0 items-center justify-between gap-2">
					<span
						className="min-w-0 truncate font-mono text-muted-foreground text-xs"
						title={type.slug}
					>
						{type.slug || "slug from name"}
					</span>
					{type.nestsInItself && (
						<span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-secondary px-1.5 py-0.5 text-[11px] text-secondary-foreground">
							<RepeatIcon aria-hidden className="size-3" />
							nests in itself
						</span>
					)}
				</div>
			</div>
			<span id={`${type.key}-description`} className="sr-only">
				{[
					type.builtIn && "Built in.",
					type.slug && `Slug ${type.slug}.`,
					type.nestsInItself && "Nests in itself.",
					trace === "path" && "On the checked path.",
					trace === "granted" &&
						`On the checked path. Grants access: ${holders.join(", ")}.`,
				]
					.filter(Boolean)
					.join(" ")}
			</span>
			{trace === "granted" && holders.length > 0 && (
				<span
					aria-hidden
					title={holders.join(", ")}
					className="pointer-events-none absolute -top-2.5 right-2 z-10 max-w-[70%] truncate rounded-full bg-success px-2 py-0.5 font-medium text-[11px] text-success-foreground shadow-xs"
				>
					◀ {holders.join(", ")}
				</span>
			)}
			{editing && !naming && (
				<button
					type="button"
					tabIndex={tabbable ? 0 : -1}
					aria-label={`Add a type under ${label}`}
					title={addBlocker ?? `Add a type under ${label}`}
					aria-disabled={addBlocker ? true : undefined}
					onClick={(event) => {
						event.stopPropagation();
						if (!addBlocker) actions.addChild(type.key, "pointer");
					}}
					className="resource-type-press nodrag nopan pointer-events-none absolute -bottom-3.5 left-1/2 z-10 -ml-3.5 flex size-7 items-center justify-center rounded-full border border-border bg-card text-muted-foreground opacity-0 shadow-xs outline-none hover:text-foreground focus-visible:pointer-events-auto focus-visible:opacity-100 focus-visible:ring-3 focus-visible:ring-ring/30 aria-disabled:cursor-not-allowed aria-disabled:text-muted-foreground/60 group-focus-within:pointer-events-auto group-focus-within:opacity-100 group-hover:pointer-events-auto group-hover:opacity-100 group-has-[[data-selected=true]]:pointer-events-auto group-has-[[data-selected=true]]:opacity-100"
				>
					<PlusIcon aria-hidden className="size-4" />
				</button>
			)}
			<Handle
				type="source"
				position={Position.Bottom}
				isConnectable={false}
				className="!pointer-events-none !opacity-0"
			/>
		</div>
	);
}
