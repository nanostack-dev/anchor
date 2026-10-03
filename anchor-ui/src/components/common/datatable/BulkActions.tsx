import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import { getApiErrorMessage } from "@/lib/api-error";
import { ChevronDown } from "lucide-react";
import { useEffect, useRef, useState } from "react";

export type DataTableBulkAction<TData> = {
	id: string;
	label: string;
	description: string;
	destructive?: boolean;
	removesRows?: boolean;
	isEligible?: (row: TData) => boolean;
	run: (row: TData) => Promise<unknown>;
};

type BulkActionsProps<TData> = {
	actions: DataTableBulkAction<TData>[];
	selectedRows: TData[];
	getRowId: (row: TData) => string;
	getRowLabel: (row: TData) => string;
	selectionScope: string;
	disabled: boolean;
	onRunningChange: (running: boolean) => void;
	onComplete: (
		succeeded: TData[],
		failed: TData[],
		action: DataTableBulkAction<TData>,
	) => Promise<void>;
};

export function BulkActions<TData>({
	actions,
	selectedRows,
	getRowId,
	getRowLabel,
	disabled,
	selectionScope,
	onRunningChange,
	onComplete,
}: BulkActionsProps<TData>) {
	const [pending, setPending] = useState<{
		action: DataTableBulkAction<TData>;
		rows: TData[];
		selectionScope: string;
	} | null>(null);
	const [progress, setProgress] = useState<number | null>(null);
	const [result, setResult] = useState<{
		succeeded: number;
		failures: { id: string; label: string; message: string }[];
		refreshError?: string;
	} | null>(null);
	const running = useRef(false);

	useEffect(() => {
		setPending((current) =>
			current?.selectionScope !== selectionScope && !running.current
				? null
				: current,
		);
	}, [selectionScope]);

	const execute = async () => {
		if (!pending || running.current) return;
		running.current = true;
		onRunningChange(true);
		setProgress(0);
		setResult(null);
		const succeeded: TData[] = [];
		const failed: TData[] = [];
		const failures: { id: string; label: string; message: string }[] = [];
		for (const row of pending.rows) {
			try {
				await pending.action.run(row);
				succeeded.push(row);
			} catch (error) {
				failed.push(row);
				failures.push({
					id: getRowId(row),
					label: getRowLabel(row),
					message:
						getApiErrorMessage(error) ??
						(error instanceof Error ? error.message : "The request failed."),
				});
			}
			setProgress(succeeded.length + failed.length);
		}
		let refreshError: string | undefined;
		try {
			await onComplete(succeeded, failed, pending.action);
		} catch {
			refreshError =
				"Couldn’t refresh the table. Reload to see the latest results.";
		} finally {
			setResult({ succeeded: succeeded.length, failures, refreshError });
			setPending(null);
			setProgress(null);
			running.current = false;
			onRunningChange(false);
		}
	};

	return (
		<>
			<div className="flex flex-wrap items-center gap-2">
				<span className="text-sm text-muted-foreground">
					{selectedRows.length} selected on this page
				</span>
				<DropdownMenu>
					<DropdownMenuTrigger
						render={
							<Button
								variant="outline"
								size="sm"
								disabled={disabled || selectedRows.length === 0}
							/>
						}
					>
						Bulk actions <ChevronDown />
					</DropdownMenuTrigger>
					<DropdownMenuContent align="start">
						{actions.map((action) => (
							<DropdownMenuItem
								key={action.id}
								variant={action.destructive ? "destructive" : "default"}
								disabled={selectedRows.some(
									(row) => !(action.isEligible?.(row) ?? true),
								)}
								onClick={() => {
									setResult(null);
									setPending({
										action,
										rows: [...selectedRows],
										selectionScope,
									});
								}}
							>
								{action.label}
							</DropdownMenuItem>
						))}
					</DropdownMenuContent>
				</DropdownMenu>
			</div>
			{result && (
				<output className="block text-sm" aria-live="polite">
					<p>
						{result.succeeded} succeeded. {result.failures.length} failed.
					</p>
					{result.failures.length > 0 && (
						<>
							<p className="text-destructive">
								Failed rows remain selected for retry.
							</p>
							<ul className="max-h-40 overflow-y-auto text-destructive">
								{result.failures.map((failure) => (
									<li key={failure.id}>
										{failure.label}: {failure.message}
									</li>
								))}
							</ul>
						</>
					)}
					{result.refreshError && (
						<p className="text-destructive">{result.refreshError}</p>
					)}
				</output>
			)}
			<AlertDialog
				open={pending !== null}
				onOpenChange={(open) => {
					if (!open && !running.current) setPending(null);
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							{pending?.action.label} ({pending?.rows.length})?
						</AlertDialogTitle>
						<AlertDialogDescription>
							{pending?.action.description}
						</AlertDialogDescription>
					</AlertDialogHeader>
					<ul className="max-h-40 overflow-y-auto text-sm">
						{pending?.rows.map((row) => (
							<li key={getRowId(row)}>{getRowLabel(row)}</li>
						))}
					</ul>
					{progress !== null && (
						<output
							className="flex items-center gap-2 text-sm"
							aria-live="polite"
						>
							<Spinner /> Processed {progress} of {pending?.rows.length}
						</output>
					)}
					<AlertDialogFooter>
						<AlertDialogCancel disabled={progress !== null}>
							Cancel
						</AlertDialogCancel>
						<AlertDialogAction
							variant={pending?.action.destructive ? "destructive" : "default"}
							disabled={progress !== null || disabled}
							onClick={() => void execute()}
						>
							{progress !== null ? "Working…" : pending?.action.label}
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</>
	);
}
