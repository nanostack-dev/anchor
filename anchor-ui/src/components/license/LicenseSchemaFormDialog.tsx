import {
	type LicenseFieldDeclaration,
	type LicenseFieldRules,
	LicenseFieldType,
	type LicenseSchemaResponse,
} from "@/client";
import {
	createLicenseSchemaMutation,
	getLicenseSchemaQueryKey,
	updateLicenseSchemaMutation,
} from "@/client/@tanstack/react-query.gen";
import { FormAlert } from "@/components/common/FormAlert";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { getApiErrorMessage, getApiFieldErrors } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Trash2 } from "lucide-react";
import { type ReactElement, useState } from "react";
import { toast } from "sonner";
import { LicenseFieldRulesEditor } from "./LicenseFieldRulesEditor";
import {
	FIELD_TYPES,
	FIELD_TYPE_LABELS,
	sanitizeRules,
} from "./license-field-format";

// `rules` is optional on the wire (an unconstrained field omits it) but always
// present in the editor's own state, so every row has something to bind
// inputs to and update in place.
type FieldRow = Omit<LicenseFieldDeclaration, "rules"> & {
	uiKey: string;
	rules: LicenseFieldRules;
};

function newRow(): FieldRow {
	return {
		uiKey: crypto.randomUUID(),
		name: "",
		type: LicenseFieldType.STRING,
		description: "",
		rules: {},
	};
}

function rowsFromSchema(schema: LicenseSchemaResponse): FieldRow[] {
	return schema.fields.map((f) => ({
		uiKey: crypto.randomUUID(),
		name: f.name,
		type: f.type,
		description: f.description ?? "",
		rules: f.rules ?? {},
	}));
}

interface LicenseSchemaFormDialogProps {
	productId: string;
	trigger: ReactElement;
	mode?: "create" | "edit";
	existingSchema?: LicenseSchemaResponse;
	onSaved?: () => void;
}

/**
 * Declares or edits a product's license schema: the fields a license may
 * carry, each field's type, and its validation rules. There is no separate
 * "required" flag — every declared field is mandatory on every template and
 * license, so the schema itself is the only place that decision is made.
 */
export function LicenseSchemaFormDialog({
	productId,
	trigger,
	mode = "create",
	existingSchema,
	onSaved,
}: LicenseSchemaFormDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const isEditMode = mode === "edit" && !!existingSchema;

	const [description, setDescription] = useState("");
	const [fields, setFields] = useState<FieldRow[]>([]);
	const [rowErrors, setRowErrors] = useState<Record<string, string>>({});
	const [listError, setListError] = useState<string | undefined>();
	const [generalError, setGeneralError] = useState<string | undefined>();

	const resetForm = () => {
		if (isEditMode && existingSchema) {
			setDescription(existingSchema.description ?? "");
			setFields(rowsFromSchema(existingSchema));
		} else {
			setDescription("");
			setFields([newRow()]);
		}
		setRowErrors({});
		setListError(undefined);
		setGeneralError(undefined);
	};

	const handleOpenChange = (next: boolean) => {
		setOpen(next);
		if (next) resetForm();
	};

	const invalidate = () => {
		queryClient.invalidateQueries({
			queryKey: getLicenseSchemaQueryKey({ path: { product_id: productId } }),
		});
	};

	const handleError = (error: unknown, fallback: string) => {
		const apiFieldErrors = getApiFieldErrors(error);
		if (Object.keys(apiFieldErrors).length > 0) {
			// API errors key by license field name; map them onto the row that
			// currently carries that name so the offending row is highlighted.
			setRowErrors(
				Object.fromEntries(
					fields
						.filter((f) => apiFieldErrors[f.name.trim()])
						.map((f) => [f.uiKey, apiFieldErrors[f.name.trim()] as string]),
				),
			);
		}
		setGeneralError(getApiErrorMessage(error) ?? fallback);
	};

	const createMutation = useMutation({
		...createLicenseSchemaMutation(),
		onSuccess: () => {
			toast.success("License schema created");
			setOpen(false);
			invalidate();
			onSaved?.();
		},
		onError: (error) => handleError(error, "Failed to create license schema."),
	});

	const updateMutation = useMutation({
		...updateLicenseSchemaMutation(),
		onSuccess: () => {
			toast.success("License schema updated");
			setOpen(false);
			invalidate();
			onSaved?.();
		},
		onError: (error) => handleError(error, "Failed to update license schema."),
	});

	const isSubmitting = createMutation.isPending || updateMutation.isPending;

	const addField = () => setFields((prev) => [...prev, newRow()]);
	const removeField = (uiKey: string) =>
		setFields((prev) => prev.filter((f) => f.uiKey !== uiKey));
	const updateField = (uiKey: string, patch: Partial<FieldRow>) =>
		setFields((prev) =>
			prev.map((f) => (f.uiKey === uiKey ? { ...f, ...patch } : f)),
		);
	const changeType = (uiKey: string, type: LicenseFieldType) =>
		setFields((prev) =>
			prev.map((f) => (f.uiKey === uiKey ? { ...f, type, rules: {} } : f)),
		);

	// Client-side pass catches the common mistakes immediately; the API
	// response is still the source of truth for anything a rule evaluator has
	// to compile (a regex) or that depends on server state (a duplicate name
	// racing another write).
	function validate(): boolean {
		const errors: Record<string, string> = {};

		if (fields.length === 0) {
			setListError("Add at least one field to declare a schema.");
			return false;
		}
		setListError(undefined);

		const namesSeen = new Map<string, string[]>();
		for (const field of fields) {
			const name = field.name.trim();
			if (!name) {
				errors[field.uiKey] = "Name is required.";
				continue;
			}
			namesSeen.set(name, [...(namesSeen.get(name) ?? []), field.uiKey]);

			if (
				field.type === LicenseFieldType.ENUM &&
				(field.rules.values?.length ?? 0) === 0
			) {
				errors[field.uiKey] = "Add at least one allowed value.";
				continue;
			}
			if (
				field.rules.min !== undefined &&
				field.rules.max !== undefined &&
				field.rules.min > field.rules.max
			) {
				errors[field.uiKey] = "Min must not exceed max.";
				continue;
			}
			if (
				field.rules.min_length !== undefined &&
				field.rules.max_length !== undefined &&
				field.rules.min_length > field.rules.max_length
			) {
				errors[field.uiKey] = "Min length must not exceed max length.";
			}
		}

		for (const uiKeys of namesSeen.values()) {
			if (uiKeys.length > 1) {
				for (const uiKey of uiKeys) {
					errors[uiKey] = "This field name is used more than once.";
				}
			}
		}

		setRowErrors(errors);
		return Object.keys(errors).length === 0;
	}

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		setGeneralError(undefined);
		if (!validate()) return;

		const declarations: LicenseFieldDeclaration[] = fields.map((field) => ({
			name: field.name.trim(),
			type: field.type,
			description: field.description?.trim() || undefined,
			rules: sanitizeRules(field.type, field.rules),
		}));

		if (isEditMode && existingSchema) {
			updateMutation.mutate({
				path: { product_id: productId },
				body: { description: description.trim(), fields: declarations },
			});
		} else {
			createMutation.mutate({
				path: { product_id: productId },
				body: { description: description.trim(), fields: declarations },
			});
		}
	};

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger render={trigger} />
			<DialogContent className="flex max-h-[90vh] flex-col p-0 sm:max-w-[680px]">
				<form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
					<DialogHeader className="px-6 pt-6">
						<DialogTitle>
							{isEditMode ? "Edit License Schema" : "Create License Schema"}
						</DialogTitle>
						<DialogDescription>
							Every field declared here becomes mandatory on every template and
							organization license for this product.
						</DialogDescription>
					</DialogHeader>

					<div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
						<FormAlert message={generalError} />

						<div className="space-y-1.5">
							<Label htmlFor="schema-description">Schema description</Label>
							<Textarea
								id="schema-description"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="What this product's license schema is for (optional)"
								rows={2}
								disabled={isSubmitting}
							/>
						</div>

						<div className="space-y-2">
							<div className="flex items-center justify-between">
								<Label>Fields</Label>
								<Button
									type="button"
									variant="outline"
									size="sm"
									onClick={addField}
									disabled={isSubmitting}
								>
									<Plus />
									Add field
								</Button>
							</div>
							{listError && (
								<p className="text-sm text-destructive">{listError}</p>
							)}

							<div className="divide-y divide-border rounded-lg border border-border">
								{fields.map((field) => (
									<div key={field.uiKey} className="flex flex-col gap-3 p-3">
										<div className="flex items-start gap-2">
											<div className="flex-1 space-y-1.5">
												<Label
													htmlFor={`${field.uiKey}-name`}
													className="text-xs"
												>
													Name
												</Label>
												<Input
													id={`${field.uiKey}-name`}
													value={field.name}
													onChange={(e) =>
														updateField(field.uiKey, { name: e.target.value })
													}
													placeholder="max_flows"
													maxLength={120}
													className="font-mono"
													disabled={isSubmitting}
												/>
											</div>
											<div className="w-36 space-y-1.5">
												<Label className="text-xs">Type</Label>
												<Select
													value={field.type}
													onValueChange={(value) =>
														changeType(field.uiKey, value as LicenseFieldType)
													}
													disabled={isSubmitting}
												>
													<SelectTrigger className="w-full">
														<SelectValue />
													</SelectTrigger>
													<SelectContent>
														{FIELD_TYPES.map((type) => (
															<SelectItem key={type} value={type}>
																{FIELD_TYPE_LABELS[type]}
															</SelectItem>
														))}
													</SelectContent>
												</Select>
											</div>
											<Button
												type="button"
												variant="outlineDestructive"
												size="icon"
												className="mt-6"
												onClick={() => removeField(field.uiKey)}
												disabled={isSubmitting}
											>
												<span className="sr-only">Remove field</span>
												<Trash2 />
											</Button>
										</div>

										<div className="space-y-1.5">
											<Label
												htmlFor={`${field.uiKey}-description`}
												className="text-xs"
											>
												Description
											</Label>
											<Input
												id={`${field.uiKey}-description`}
												value={field.description ?? ""}
												onChange={(e) =>
													updateField(field.uiKey, {
														description: e.target.value,
													})
												}
												placeholder="Optional"
												disabled={isSubmitting}
											/>
										</div>

										<LicenseFieldRulesEditor
											idPrefix={field.uiKey}
											type={field.type}
											rules={field.rules}
											onChange={(rules) => updateField(field.uiKey, { rules })}
											disabled={isSubmitting}
										/>

										{rowErrors[field.uiKey] && (
											<p className="text-sm text-destructive">
												{rowErrors[field.uiKey]}
											</p>
										)}
									</div>
								))}
							</div>
						</div>
					</div>

					<DialogFooter className="border-t border-border px-6 py-4">
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
							disabled={isSubmitting}
						>
							Cancel
						</Button>
						<Button type="submit" disabled={isSubmitting}>
							{isSubmitting
								? isEditMode
									? "Saving…"
									: "Creating…"
								: isEditMode
									? "Save Schema"
									: "Create Schema"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
