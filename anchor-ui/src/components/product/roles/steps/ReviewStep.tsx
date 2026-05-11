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
import { VerticalStepperStep } from "@/components/ui/vertical-stepper";
import {
	CheckCircle2,
	ChevronLeft,
	Edit,
	Loader2,
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
					<DialogHeader className="space-y-2">
						<DialogTitle className="flex items-center gap-3">
							<div className="p-2 rounded-xl bg-slate-900 text-white shadow-sm">
								<ShieldCheck className="h-4 w-4" />
							</div>
							<span className="text-xl font-semibold tracking-tight text-slate-900">
								Review
							</span>
							{isEditMode && (
								<Badge
									variant="outline"
									className="ml-1 text-xs font-normal border-slate-200 text-slate-500 bg-slate-50"
								>
									Editing: {existingRole?.name}
								</Badge>
							)}
						</DialogTitle>
						<DialogDescription className="text-sm text-slate-500 leading-relaxed">
							Review your configuration before{" "}
							{isEditMode ? "saving changes" : "creating the role"}.
						</DialogDescription>
					</DialogHeader>
				</div>

				<ScrollArea className="flex-1 px-7">
					<div className="space-y-5 pb-6">
						{/* Ready banner */}
						<div className="flex items-center gap-3 p-4 rounded-xl border border-emerald-200 bg-emerald-50">
							<CheckCircle2 className="h-5 w-5 text-emerald-500 shrink-0" />
							<div>
								<p className="text-sm font-semibold text-emerald-800">
									Ready to {isEditMode ? "update" : "create"}
								</p>
								<p className="text-xs text-emerald-600 mt-0.5">
									Everything looks good. Confirm the details below.
								</p>
							</div>
						</div>

						{/* Role Name */}
						<div className="rounded-xl border border-slate-200 bg-white overflow-hidden">
							<div className="px-4 py-3 border-b border-slate-100 bg-slate-50/60">
								<Label className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
									Role Name
								</Label>
							</div>
							<div className="px-4 py-4">
								<p className="text-xl font-semibold text-slate-900 tracking-tight">
									{formData.name}
								</p>
							</div>
						</div>

						{/* Description */}
						{formData.description && (
							<div className="rounded-xl border border-slate-200 bg-white overflow-hidden">
								<div className="px-4 py-3 border-b border-slate-100 bg-slate-50/60">
									<Label className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
										Description
									</Label>
								</div>
								<div className="px-4 py-4">
									<p className="text-sm text-slate-700 leading-relaxed whitespace-pre-wrap break-words">
										{formData.description}
									</p>
								</div>
							</div>
						)}

						{/* Permissions */}
						<div className="rounded-xl border border-slate-200 bg-white overflow-hidden">
							<div className="px-4 py-3 border-b border-slate-100 bg-slate-50/60 flex items-center justify-between">
								<Label className="text-xs font-semibold text-slate-500 uppercase tracking-wider">
									Permissions
								</Label>
								<Badge
									variant="secondary"
									className="text-xs bg-slate-100 text-slate-600 border-0"
								>
									{formData.selectedPermissions.length} selected
								</Badge>
							</div>
							<div className="px-4 py-4">
								{formData.selectedPermissions.length === 0 ? (
									<p className="text-sm text-slate-400 italic">
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
															? "bg-slate-50 border-slate-200 text-slate-700"
															: "bg-emerald-50 border-emerald-200 text-emerald-700"
													}`}
												>
													{permission}
													{!isOriginal && isEditMode && (
														<span className="ml-1.5 font-sans text-emerald-500 not-italic">
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
							<div className="rounded-xl border border-red-200 bg-white overflow-hidden">
								<div className="px-4 py-3 border-b border-red-100 bg-red-50/60 flex items-center justify-between">
									<Label className="text-xs font-semibold text-red-500 uppercase tracking-wider">
										Permissions to Remove
									</Label>
									<Badge
										variant="outline"
										className="text-xs bg-red-50 border-red-200 text-red-600"
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
												className="text-xs px-2.5 py-1 font-mono rounded-lg bg-red-50 border-red-200 text-red-600"
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
				<div className="px-7 py-5 border-t border-slate-100 mt-auto">
					<DialogFooter className="p-0">
						<div className="flex items-center justify-between w-full">
							<Button
								type="button"
								variant="ghost"
								onClick={onPrevious}
								className="px-4 h-10 rounded-xl text-slate-600 hover:text-slate-900 hover:bg-slate-100"
							>
								<ChevronLeft className="mr-1.5 h-4 w-4" />
								Back
							</Button>

							<div className="flex items-center gap-2">
								<Button
									type="button"
									variant="outline"
									onClick={onCancel}
									className="px-5 h-10 rounded-xl border-slate-200 text-slate-600 hover:bg-slate-50"
								>
									Cancel
								</Button>

								<Button
									onClick={onSubmit}
									disabled={isLoading}
									className="px-6 h-10 rounded-xl bg-slate-900 hover:bg-slate-800 text-white font-medium shadow-sm transition-all"
								>
									{isLoading ? (
										<>
											<Loader2 className="mr-2 h-4 w-4 animate-spin" />
											{isEditMode ? "Saving..." : "Creating..."}
										</>
									) : (
										<>
											{isEditMode ? (
												<Edit className="mr-2 h-4 w-4" />
											) : (
												<Plus className="mr-2 h-4 w-4" />
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
