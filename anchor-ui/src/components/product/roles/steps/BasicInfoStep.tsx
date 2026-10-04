import type { ProductRoleResponse } from "@/client";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import { ScrollArea } from "@nanostackorg/design-system/components/scroll-area";
import { TabsContent } from "@nanostackorg/design-system/components/tabs";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { Box } from "@nanostackorg/design-system/layout/box";
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
		<TabsContent value="basic">
			<Box className="flex flex-col h-full">
				{/* Header */}
				<Box className="px-7 pt-7 pb-5">
					<Box className="flex flex-col gap-2">
						<h2 className="flex items-center gap-3">
							<Box className="p-2 rounded-xl bg-primary text-primary-foreground shadow-sm">
								<Lock className="size-4" />
							</Box>
							<Box
								as="span"
								className="text-xl font-semibold tracking-tight text-foreground"
							>
								Basic Info
							</Box>
							{isEditMode && (
								<Badge variant="outline">Editing: {existingRole?.name}</Badge>
							)}
						</h2>
						<Box
							as="p"
							className="text-sm text-muted-foreground leading-relaxed"
						>
							{isEditMode
								? "Update the basic information for this role."
								: "Give your new role a clear name and description."}
						</Box>
					</Box>
				</Box>

				<ScrollArea maxHeight="md">
					<Box className="flex flex-col gap-6 pb-6">
						{/* Role Name */}
						<form.Field name="name">
							{(field) => (
								<Box className="flex flex-col gap-2">
									<Label htmlFor="name">
										Role Name{" "}
										<Box as="span" className="text-destructive">
											*
										</Box>
									</Label>
									<Input
										id="name"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										placeholder="e.g., Content Editor, Admin, Viewer"
									/>
									<FormValidationError field={field} />
								</Box>
							)}
						</form.Field>

						{/* Description */}
						<form.Field name="description">
							{(field) => (
								<Box className="flex flex-col gap-2">
									<Label htmlFor="description">
										Description
										<Box
											as="span"
											className="ml-1.5 text-xs font-normal text-muted-foreground"
										>
											Optional
										</Box>
									</Label>
									<Textarea
										id="description"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										placeholder="Describe what this role can do and who should have it..."
										rows={4}
									/>
									<FormValidationError field={field} />
								</Box>
							)}
						</form.Field>

						{/* Tip card */}
						<Box className="flex items-start gap-3 p-4 rounded-xl border border-border bg-muted">
							<Box className="mt-0.5 p-1.5 rounded-lg bg-warning/10">
								<Lightbulb className="size-3.5 text-warning-on-tint" />
							</Box>
							<Box className="flex flex-col gap-1">
								<Box as="p" className="text-xs font-semibold text-foreground">
									Role Naming Best Practices
								</Box>
								<Box
									as="p"
									className="text-xs text-muted-foreground leading-relaxed"
								>
									Use descriptive names like "Content Editor" or "Analytics
									Viewer". Clear names help team members understand permissions
									at a glance.
								</Box>
							</Box>
						</Box>
					</Box>
				</ScrollArea>

				{/* Footer */}
				<Box className="px-7 py-5 border-t border-border mt-auto">
					<Box className="p-0">
						<Box className="flex justify-end w-full">
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
										variant="solid"
										tone="brand"
										onClick={form.handleSubmit}
										disabled={
											!canSubmit ||
											isSubmitting ||
											!isValid ||
											(!isEditMode && !isDirty)
										}
									>
										Continue
										<ChevronRight className="ml-1.5 size-4" />
									</Button>
								)}
							</form.Subscribe>
						</Box>
					</Box>
				</Box>
			</Box>
		</TabsContent>
	);
}
