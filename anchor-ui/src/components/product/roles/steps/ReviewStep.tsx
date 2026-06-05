import type { ProductRoleResponse } from "@/client";
import type { RoleFormData } from "@/components/product/roles/form-type";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Spinner } from "@/components/ui/spinner";
import { VerticalStepperStep } from "@/components/ui/vertical-stepper";
import {
	CheckCircle2,
	ChevronLeft,
	Edit,
	Plus,
	ShieldCheck,
} from "lucide-react";

interface ReviewStepProps {
	formData: RoleFormData;
	isEditMode: boolean;
	existingRole?: ProductRoleResponse;
	isLoading: boolean;
	onSubmit: () => void;
	onPrevious: () => void;
	onCancel: () => void;
}

export function ReviewStep({
	formData,
	isEditMode,
	existingRole,
	isLoading,
	onSubmit,
	onPrevious,
	onCancel,
}: ReviewStepProps) {
	const removedPermissions =
		isEditMode && existingRole?.permissions
			? existingRole.permissions
					.map((p) => p.permission_name)
					.filter(
						(permName) => !formData.selectedPermissions.includes(permName),
					)
			: [];

	return (
		<VerticalStepperStep id="review">
			<div className="flex flex-col h-full">
				{/* Header */}
				<div className="px-7 pt-7 pb-5">
					<DialogHeader className="gap-2">
						<DialogTitle className="flex items-center gap-3">
							<div className="p-2 rounded-xl bg-primary text-primary-foreground shadow-sm">
								<ShieldCheck className="size-4" />
							</div>
							<span className="text-xl font-semibold tracking-tight text-foreground">
								Review
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
							Review your configuration before{" "}
							{isEditMode ? "saving changes" : "creating the role"}.
						</DialogDescription>
					</DialogHeader>
				</div>

				<ScrollArea className="flex-1 px-7">
					<div className="flex flex-col gap-5 pb-6">
						{/* Ready banner */}
						<div className="flex items-center gap-3 p-4 rounded-xl border border-success/30 bg-success/10">
							<CheckCircle2 className="size-5 text-success shrink-0" />
							<div>
								<p className="text-sm font-semibold text-success">
									Ready to {isEditMode ? "update" : "create"}
								</p>
								<p className="text-xs text-success mt-0.5">
									Everything looks good. Confirm the details below.
								</p>
							</div>
						</div>

						{/* Role Name */}
						<div className="rounded-xl border border-border bg-card overflow-hidden">
							<div className="px-4 py-3 border-b border-border bg-muted/60">
								<Label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
									Role Name
								</Label>
							</div>
							<div className="px-4 py-4">
								<p className="text-xl font-semibold text-foreground tracking-tight">
									{formData.name}
								</p>
							</div>
						</div>

						{/* Description */}
						{formData.description && (
							<div className="rounded-xl border border-border bg-card overflow-hidden">
								<div className="px-4 py-3 border-b border-border bg-muted/60">
									<Label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
										Description
									</Label>
								</div>
								<div className="px-4 py-4">
									<p className="text-sm text-foreground leading-relaxed whitespace-pre-wrap break-words">
										{formData.description}
									</p>
								</div>
							</div>
						)}

						{/* Permissions */}
						<div className="rounded-xl border border-border bg-card overflow-hidden">
							<div className="px-4 py-3 border-b border-border bg-muted/60 flex items-center justify-between">
								<Label className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
									Permissions
								</Label>
								<Badge variant="secondary" className="text-xs border-0">
									{formData.selectedPermissions.length} selected
								</Badge>
							</div>
							<div className="px-4 py-4">
								{formData.selectedPermissions.length === 0 ? (
									<p className="text-sm text-muted-foreground italic">
										No permissions selected
									</p>
								) : (
									<div className="flex flex-wrap gap-1.5">
										{formData.selectedPermissions.map((permission) => {
											const isOriginal =
												isEditMode &&
												existingRole?.permissions?.some(
													(p) => p.permission_name === permission,
												);
											return (
												<Badge
													key={permission}
													variant="outline"
													className={`text-xs px-2.5 py-1 font-mono rounded-lg ${
														isOriginal
															? "bg-muted border-border text-foreground"
															: "bg-success/10 border-success/30 text-success"
													}`}
												>
													{permission}
													{!isOriginal && isEditMode && (
														<span className="ml-1.5 font-sans text-success not-italic">
															New
														</span>
													)}
												</Badge>
											);
										})}
									</div>
								)}
							</div>
						</div>

						{/* Removed permissions (edit mode only) */}
						{removedPermissions.length > 0 && (
							<div className="rounded-xl border border-destructive/30 bg-card overflow-hidden">
								<div className="px-4 py-3 border-b border-destructive/20 bg-destructive/10 flex items-center justify-between">
									<Label className="text-xs font-semibold text-destructive uppercase tracking-wider">
										Permissions to Remove
									</Label>
									<Badge
										variant="outline"
										className="text-xs bg-destructive/10 border-destructive/30 text-destructive"
									>
										{removedPermissions.length}
									</Badge>
								</div>
								<div className="px-4 py-4">
									<div className="flex flex-wrap gap-1.5">
										{removedPermissions.map((permission) => (
											<Badge
												key={permission}
												variant="outline"
												className="text-xs px-2.5 py-1 font-mono rounded-lg bg-destructive/10 border-destructive/30 text-destructive"
											>
												{permission}
											</Badge>
										))}
									</div>
								</div>
							</div>
						)}
					</div>
				</ScrollArea>

				{/* Footer */}
				<div className="px-7 py-5 border-t border-border mt-auto">
					<DialogFooter className="p-0">
						<div className="flex items-center justify-between w-full">
							<Button
								type="button"
								variant="ghost"
								onClick={onPrevious}
								className="px-4 h-10 rounded-xl"
							>
								<ChevronLeft className="mr-1.5 size-4" />
								Back
							</Button>

							<div className="flex items-center gap-2">
								<Button
									type="button"
									variant="outline"
									onClick={onCancel}
									className="px-5 h-10 rounded-xl"
								>
									Cancel
								</Button>

								<Button
									onClick={onSubmit}
									disabled={isLoading}
									className="px-6 h-10 rounded-xl font-medium shadow-sm transition-all"
								>
									{isLoading ? (
										<>
											<Spinner className="mr-2 text-current" />
											{isEditMode ? "Saving..." : "Creating..."}
										</>
									) : (
										<>
											{isEditMode ? (
												<Edit className="mr-2 size-4" />
											) : (
												<Plus className="mr-2 size-4" />
											)}
											{isEditMode ? "Save Changes" : "Create Role"}
										</>
									)}
								</Button>
							</div>
						</div>
					</DialogFooter>
				</div>
			</div>
		</VerticalStepperStep>
	);
}
