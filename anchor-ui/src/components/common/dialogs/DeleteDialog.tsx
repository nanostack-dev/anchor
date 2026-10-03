import { getApiErrorMessage } from "@/lib/api-error";
import {
	Alert,
	AlertDescription,
} from "@nanostackorg/design-system/components/alert";
import { IconButton } from "@nanostackorg/design-system/components/button";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@nanostackorg/design-system/components/dialog";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { toast } from "@nanostackorg/design-system/components/toast";
import { TrashIcon } from "@phosphor-icons/react";
import { useMutation } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { type ReactElement, type ReactNode, useState } from "react";
import { FormAlert } from "../FormAlert";

interface DeleteDialogProps {
	trigger?: ReactElement;
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
			toast.add({
				type: "success",
				title: `${entityType} deleted successfully!`,
			});
			setOpen(false);
			onDeleted?.();
		},
		onError: (error: unknown) => {
			console.error(`Failed to delete ${entityType.toLowerCase()}:`, error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.add({ type: "error", title: errorMessage });
			} else {
				toast.add({
					type: "error",
					title: `Failed to delete ${entityType.toLowerCase()}. Please try again.`,
				});
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
		<Alert tone="warning">
			<AlertDescription>{warningMessage}</AlertDescription>
		</Alert>
	);

	return (
		<Dialog
			open={open}
			onOpenChange={(nextOpen) => {
				if (!deleteMutation.isPending) setOpen(nextOpen);
			}}
		>
			<DialogTrigger
				render={
					trigger ? (
						trigger
					) : (
						<IconButton
							icon={TrashIcon}
							label={`Delete ${entityType}`}
							tone="critical"
							variant="outline"
							disabled={disabled}
							loading={deleteMutation.isPending}
						/>
					)
				}
			/>
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
						variant="soft"
						tone="critical"
						onClick={handleDelete}
						disabled={deleteMutation.isPending}
					>
						{deleteMutation.isPending ? (
							<>
								<Spinner />
								Deleting...
							</>
						) : (
							<>
								<Trash2 className="mr-2 size-4" />
								Delete {entityType}
							</>
						)}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
