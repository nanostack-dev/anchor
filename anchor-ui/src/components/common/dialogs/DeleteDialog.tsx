import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation } from "@tanstack/react-query";
import { Loader2, Trash2 } from "lucide-react";
import { type ReactNode, useState } from "react";
import { toast } from "sonner";
import { FormAlert } from "../FormAlert";

interface DeleteDialogProps {
	trigger?: ReactNode;
	entityType: string;
	entityName: string;
	displayFields: Array<{
		label: string;
		value: string | ReactNode;
		condition?: boolean;
	}>;
	warningMessage?: string;
	onDelete: () => Promise<void>;
	onDeleted?: () => void;
	disabled?: boolean;
}

export function DeleteDialog({
	trigger,
	entityType,
	entityName,
	displayFields,
	warningMessage,
	onDelete,
	onDeleted,
	disabled = false,
}: DeleteDialogProps) {
	const [open, setOpen] = useState(false);

	const deleteMutation = useMutation({
		mutationFn: onDelete,
		onSuccess: () => {
			toast.success(`${entityType} deleted successfully!`);
			setOpen(false);
			onDeleted?.();
		},
		onError: (error: unknown) => {
			console.error(`Failed to delete ${entityType.toLowerCase()}:`, error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.error(errorMessage);
			} else {
				toast.error(
					`Failed to delete ${entityType.toLowerCase()}. Please try again.`,
				);
			}
		},
	});

	const handleDelete = () => {
		deleteMutation.mutate();
	};

	const displayFieldsContent = (
		<div className="my-4 p-4 bg-muted rounded-lg">
			<div className="space-y-2">
				{displayFields
					.filter((field) => field.condition !== false)
					.map((field) => (
						<div
							key={field.label}
							className="flex justify-between items-start gap-4"
						>
							<span className="font-medium">{field.label}:</span>
							<span
								className={[
									typeof field.value === "string" &&
									field.label.toLowerCase().includes("id")
										? "font-mono text-sm"
										: "",
									"break-all max-w-xs text-right",
								].join(" ")}
								title={
									typeof field.value === "string" ? field.value : undefined
								}
							>
								{field.value}
							</span>
						</div>
					))}
			</div>
		</div>
	);

	const warningContent = warningMessage && (
		<div className="p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
			<p className="text-sm text-yellow-800">
				<strong>Warning:</strong> {warningMessage}
			</p>
		</div>
	);

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>
				{trigger ? (
					trigger
				) : (
					<Button disabled={disabled} variant="outlineDestructive" size="icon">
						<span className="sr-only">Delete {entityType}</span>
						{deleteMutation.isPending ? (
							<Loader2 className="h-4 w-4 animate-spin" />
						) : (
							<Trash2 className="h-4 w-4" />
						)}
					</Button>
				)}
			</DialogTrigger>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Delete {entityType}</DialogTitle>
					<DialogDescription>
						Are you sure you want to delete the {entityType.toLowerCase()} "
						{entityName}"? This action cannot be undone and will permanently
						remove the {entityType.toLowerCase()}.
					</DialogDescription>
				</DialogHeader>

				{displayFieldsContent}
				{warningContent}

				{deleteMutation.error ? (
					<FormAlert
						variant="default"
						message={
							getApiErrorMessage(deleteMutation.error) ||
							`Failed to delete ${entityType.toLowerCase()}`
						}
					/>
				) : null}

				<DialogFooter>
					<Button
						variant="outline"
						onClick={() => setOpen(false)}
						disabled={deleteMutation.isPending}
					>
						Cancel
					</Button>
					<Button
						variant="destructive"
						onClick={handleDelete}
						disabled={deleteMutation.isPending}
					>
						{deleteMutation.isPending ? (
							<>
								<Loader2 className="mr-2 h-4 w-4 animate-spin" />
								Deleting...
							</>
						) : (
							<>
								<Trash2 className="mr-2 h-4 w-4" />
								Delete {entityType}
							</>
						)}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
