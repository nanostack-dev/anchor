import type { ProductRoleResponse } from "@/client";
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
import { Textarea } from "@/components/ui/textarea";
import { VerticalStepperStep } from "@/components/ui/vertical-stepper";
import { useForm } from "@tanstack/react-form";
import { ChevronRight, Lightbulb, Lock } from "lucide-react";
import { type BasicInfoFormData, basicInfo } from "../form-type";

interface BasicInfoStepProps {
	initialData: BasicInfoFormData;
	onFormDataChange: (data: BasicInfoFormData) => void;
	isEditMode: boolean;
	existingRole?: ProductRoleResponse;
	onNext?: () => void;
}

export function BasicInfoStep({
	initialData,
	onFormDataChange,
	isEditMode,
	existingRole,
	onNext,
}: BasicInfoStepProps) {
	const form = useForm({
		defaultValues: initialData,
		onSubmit: async ({ value }) => {
			onFormDataChange(value);
			if (onNext) {
				onNext();
			}
		},
		validators: {
			onChange: basicInfo,
			onSubmit: basicInfo,
		},
	});
	if (isEditMode && !existingRole) {
		throw new Error("Existing role must be provided in edit mode");
	}
	return (
		<VerticalStepperStep id="basic">
			<div className="flex flex-col h-full">
				{/* Header */}
				<div className="px-7 pt-7 pb-5">
					<DialogHeader className="gap-2">
						<DialogTitle className="flex items-center gap-3">
							<div className="p-2 rounded-xl bg-primary text-primary-foreground shadow-sm">
								<Lock className="size-4" />
							</div>
							<span className="text-xl font-semibold tracking-tight text-foreground">
								Basic Info
							</span>
							{isEditMode && (
								<Badge
									variant="outline"
									className="ml-1 text-xs font-normal text-muted-foreground bg-muted"
								>
									Editing: {existingRole?.name}
								</Badge>
							)}
						</DialogTitle>
						<DialogDescription className="text-sm text-muted-foreground leading-relaxed">
							{isEditMode
								? "Update the basic information for this role."
								: "Give your new role a clear name and description."}
						</DialogDescription>
					</DialogHeader>
				</div>

				<ScrollArea className="flex-1 px-7">
					<div className="flex flex-col gap-6 pb-6">
						{/* Role Name */}
						<form.Field name="name">
							{(field) => (
								<div className="flex flex-col gap-2">
									<Label
										htmlFor="name"
										className="text-sm font-medium text-foreground"
									>
										Role Name <span className="text-destructive">*</span>
									</Label>
									<Input
										id="name"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										placeholder="e.g., Content Editor, Admin, Viewer"
										className="h-11 text-base rounded-xl"
									/>
									<FormValidationError field={field} />
								</div>
							)}
						</form.Field>

						{/* Description */}
						<form.Field name="description">
							{(field) => (
								<div className="flex flex-col gap-2">
									<Label
										htmlFor="description"
										className="text-sm font-medium text-foreground"
									>
										Description
										<span className="ml-1.5 text-xs font-normal text-muted-foreground">
											Optional
										</span>
									</Label>
									<Textarea
										id="description"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										placeholder="Describe what this role can do and who should have it..."
										rows={4}
										className="resize-none text-sm rounded-xl leading-relaxed"
									/>
									<FormValidationError field={field} />
								</div>
							)}
						</form.Field>

						{/* Tip card */}
						<div className="flex items-start gap-3 p-4 rounded-xl border border-border bg-muted">
							<div className="mt-0.5 p-1.5 rounded-lg bg-warning/10">
								<Lightbulb className="size-3.5 text-warning" />
							</div>
							<div className="flex flex-col gap-1">
								<p className="text-xs font-semibold text-foreground">
									Role Naming Best Practices
								</p>
								<p className="text-xs text-muted-foreground leading-relaxed">
									Use descriptive names like "Content Editor" or "Analytics
									Viewer". Clear names help team members understand permissions
									at a glance.
								</p>
							</div>
						</div>
					</div>
				</ScrollArea>

				{/* Footer */}
				<div className="px-7 py-5 border-t border-border mt-auto">
					<DialogFooter className="p-0">
						<div className="flex justify-end w-full">
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
										className="px-6 h-10 rounded-xl font-medium shadow-sm transition-all"
									>
										Continue
										<ChevronRight className="ml-1.5 size-4" />
									</Button>
								)}
							</form.Subscribe>
						</div>
					</DialogFooter>
				</div>
			</div>
		</VerticalStepperStep>
	);
}
