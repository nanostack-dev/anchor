import type { ProductApiKeyResponse } from "@/client";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { VerticalStepperStep } from "@/components/ui/vertical-stepper";
import { useForm } from "@tanstack/react-form";
import { AlertCircle, ChevronRight, Key } from "lucide-react";
import { type BasicInfoFormData, basicInfo } from "../form-type";

interface BasicInfoStepProps {
	initialData: BasicInfoFormData;
	onFormDataChange: (data: BasicInfoFormData) => void;
	isEditMode: boolean;
	apiKey?: ProductApiKeyResponse;
	onNext: () => void;
	onCancel: () => void;
}

export function BasicInfoStep({
	initialData,
	onFormDataChange,
	isEditMode,
	apiKey,
	onNext,
	onCancel,
}: BasicInfoStepProps) {
	const form = useForm({
		defaultValues: initialData,
		onSubmit: async ({ value }) => {
			const result = basicInfo.safeParse(value);
			if (!result.success) {
				return;
			}
			onFormDataChange(value);
			onNext();
		},
		validators: {
			onChange: basicInfo,
			onSubmit: basicInfo,
		},
	});

	if (isEditMode && !apiKey) {
		throw new Error("API key must be provided in edit mode");
	}

	return (
		<VerticalStepperStep id="basic">
			<div className="flex flex-col h-full">
				<div className="px-6 pt-6 pb-4">
					<DialogHeader className="space-y-3">
						<DialogTitle className="flex items-center gap-3">
							<div className="p-2 rounded-lg bg-primary text-primary-foreground">
								<Key className="size-5" />
							</div>
							<span className="text-xl">Basic Info</span>
							{isEditMode && (
								<Badge variant="outline" className="ml-2">
									Editing: {apiKey?.name}
								</Badge>
							)}
						</DialogTitle>
						<DialogDescription className="text-base">
							{`${isEditMode ? "Update" : "Define"} the basic information for your ${isEditMode ? "existing" : "new"} API key`}
						</DialogDescription>
					</DialogHeader>
				</div>

				<ScrollArea className="flex-1 px-6">
					<div className="space-y-6 pb-6">
						<form.Field name="name">
							{(field) => (
								<div className="space-y-3">
									<Label htmlFor="name" className="text-sm font-semibold">
										API Key Name <span className="text-destructive">*</span>
									</Label>
									<Input
										id="name"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										placeholder="e.g., Production Key, Development Key, Analytics API"
										className="text-lg h-12"
									/>
									<FormValidationError field={field} />
								</div>
							)}
						</form.Field>

						<form.Field name="description">
							{(field) => (
								<div className="space-y-3">
									<Label
										htmlFor="description"
										className="text-sm font-semibold"
									>
										Description
									</Label>
									<Textarea
										id="description"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										placeholder="Describe the purpose of this API key and who should have access..."
										rows={4}
										className="resize-none text-base whitespace-pre-wrap break-words"
									/>
									<FormValidationError field={field} />
								</div>
							)}
						</form.Field>

						{!isEditMode && (
							<form.Field name="mutable">
								{(field) => (
									<div className="space-y-3">
										<div className="flex items-center justify-between rounded-lg border p-4">
											<div className="space-y-1">
												<Label
													htmlFor="mutable"
													className="text-sm font-semibold"
												>
													Mutable permissions
												</Label>
												<p className="text-xs text-muted-foreground">
													Allow updating this API key permissions after
													creation.
												</p>
											</div>
											<Switch
												id="mutable"
												checked={field.state.value}
												onCheckedChange={field.handleChange}
											/>
										</div>
										<FormValidationError field={field} />
									</div>
								)}
							</form.Field>
						)}

						{isEditMode &&
							apiKey?.permissions &&
							apiKey.permissions.length > 0 && (
								<div className="p-4 rounded-xl border border-border bg-muted/50">
									<div className="space-y-3">
										<Label className="text-sm font-semibold text-foreground">
											Current Permissions ({apiKey.permissions.length})
										</Label>
										<div className="flex flex-wrap gap-2">
											{apiKey.permissions.map((permission) => (
												<Badge
													key={permission.permission_name}
													variant="outline"
													className="text-sm px-3 py-1 bg-primary/10 border-primary/20 text-primary"
												>
													<code>{permission.permission_name}</code>
												</Badge>
											))}
										</div>
										<p className="text-xs text-muted-foreground">
											This API key is{" "}
											{apiKey?.mutable ? "mutable" : "immutable"} for permission
											updates.
										</p>
									</div>
								</div>
							)}

						{!isEditMode && (
							<div className="p-4 rounded-xl border border-border bg-muted/50">
								<div className="flex items-start gap-3">
									<div className="p-2 rounded-lg bg-primary/10">
										<AlertCircle className="size-5 text-primary" />
									</div>
									<div>
										<p className="text-sm font-semibold text-foreground">
											💡 API Key Security Best Practices
										</p>
										<p className="text-sm mt-2 text-muted-foreground">
											Use descriptive names and store keys securely. API keys
											provide programmatic access to your product data.
										</p>
									</div>
								</div>
							</div>
						)}
					</div>
				</ScrollArea>
				<div className="px-6 py-6 border-t mt-auto">
					<DialogFooter className="p-0">
						<div className="flex justify-between w-full">
							<div />

							<div className="flex gap-3">
								<Button
									type="button"
									variant="outline"
									onClick={onCancel}
									className="px-6"
								>
									Cancel
								</Button>

								<form.Subscribe
									selector={(state) => [
										state.canSubmit,
										state.isSubmitting,
										state.isValid,
										state.isDirty,
									]}
								>
									{([canSubmit, isSubmitting, isValid, isDirty]) => (
										<Button
											onClick={form.handleSubmit}
											disabled={
												!canSubmit ||
												isSubmitting ||
												!isValid ||
												(!isEditMode && !isDirty)
											}
											className="px-6"
										>
											Next
											<ChevronRight data-icon="inline-end" className="size-4" />
										</Button>
									)}
								</form.Subscribe>
							</div>
						</div>
					</DialogFooter>
				</div>
			</div>
		</VerticalStepperStep>
	);
}
