import type { ProductRoleResponse } from "@/client";
import type { RoleFormData } from "@/components/product/roles/form-type";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import { Label } from "@nanostackorg/design-system/components/label";
import { ScrollArea } from "@nanostackorg/design-system/components/scroll-area";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { TabsContent } from "@nanostackorg/design-system/components/tabs";
import { Box } from "@nanostackorg/design-system/layout/box";
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
		<TabsContent value="review">
			<Box className="flex flex-col h-full">
				{/* Header */}
				<Box className="px-7 pt-7 pb-5">
					<Box className="flex flex-col gap-2">
						<h2 className="flex items-center gap-3">
							<Box className="p-2 rounded-xl bg-primary text-primary-foreground shadow-sm">
								<ShieldCheck className="size-4" />
							</Box>
							<Box
								as="span"
								className="text-xl font-semibold tracking-tight text-foreground"
							>
								Review
							</Box>
							{isEditMode && (
								<Badge variant="outline">Editing: {existingRole?.name}</Badge>
							)}
						</h2>
						<Box
							as="p"
							className="text-sm text-muted-foreground leading-relaxed"
						>
							Review your configuration before{" "}
							{isEditMode ? "saving changes" : "creating the role"}.
						</Box>
					</Box>
				</Box>

				<ScrollArea maxHeight="md">
					<Box className="flex flex-col gap-5 pb-6">
						{/* Ready banner */}
						<Box className="flex items-center gap-3 p-4 rounded-xl border border-success/30 bg-success/10">
							<CheckCircle2 className="size-5 text-success-on-tint shrink-0" />
							<div>
								<Box
									as="p"
									className="text-sm font-semibold text-success-on-tint"
								>
									Ready to {isEditMode ? "update" : "create"}
								</Box>
								<Box as="p" className="text-xs text-success-on-tint mt-0.5">
									Everything looks good. Confirm the details below.
								</Box>
							</div>
						</Box>

						{/* Role Name */}
						<Box className="rounded-xl border border-border bg-card overflow-hidden">
							<Box className="px-4 py-3 border-b border-border bg-muted/60">
								<Label>Role Name</Label>
							</Box>
							<Box className="px-4 py-4">
								<Box
									as="p"
									className="text-xl font-semibold text-foreground tracking-tight"
								>
									{formData.name}
								</Box>
							</Box>
						</Box>

						{/* Description */}
						{formData.description && (
							<Box className="rounded-xl border border-border bg-card overflow-hidden">
								<Box className="px-4 py-3 border-b border-border bg-muted/60">
									<Label>Description</Label>
								</Box>
								<Box className="px-4 py-4">
									<Box
										as="p"
										className="text-sm text-foreground leading-relaxed whitespace-pre-wrap break-words"
									>
										{formData.description}
									</Box>
								</Box>
							</Box>
						)}

						{/* Permissions */}
						<Box className="rounded-xl border border-border bg-card overflow-hidden">
							<Box className="px-4 py-3 border-b border-border bg-muted/60 flex items-center justify-between">
								<Label>Permissions</Label>
								<Badge tone="neutral" variant="soft">
									{formData.selectedPermissions.length} selected
								</Badge>
							</Box>
							<Box className="px-4 py-4">
								{formData.selectedPermissions.length === 0 ? (
									<Box as="p" className="text-sm text-muted-foreground italic">
										No permissions selected
									</Box>
								) : (
									<Box className="flex flex-wrap gap-1.5">
										{formData.selectedPermissions.map((permission) => {
											const isOriginal =
												isEditMode &&
												existingRole?.permissions?.some(
													(p) => p.permission_name === permission,
												);
											return (
												<Badge key={permission} variant="outline">
													{permission}
													{!isOriginal && isEditMode && (
														<Box
															as="span"
															className="ml-1.5 font-sans text-success-on-tint not-italic"
														>
															New
														</Box>
													)}
												</Badge>
											);
										})}
									</Box>
								)}
							</Box>
						</Box>

						{/* Removed permissions (edit mode only) */}
						{removedPermissions.length > 0 && (
							<Box className="rounded-xl border border-destructive/30 bg-card overflow-hidden">
								<Box className="px-4 py-3 border-b border-destructive/20 bg-destructive/10 flex items-center justify-between">
									<Label>Permissions to Remove</Label>
									<Badge variant="outline">{removedPermissions.length}</Badge>
								</Box>
								<Box className="px-4 py-4">
									<Box className="flex flex-wrap gap-1.5">
										{removedPermissions.map((permission) => (
											<Badge key={permission} variant="outline">
												{permission}
											</Badge>
										))}
									</Box>
								</Box>
							</Box>
						)}
					</Box>
				</ScrollArea>

				{/* Footer */}
				<Box className="px-7 py-5 border-t border-border mt-auto">
					<Box className="p-0">
						<Box className="flex items-center justify-between w-full">
							<Button type="button" variant="ghost" onClick={onPrevious}>
								<ChevronLeft className="mr-1.5 size-4" />
								Back
							</Button>

							<Box className="flex items-center gap-2">
								<Button type="button" variant="outline" onClick={onCancel}>
									Cancel
								</Button>

								<Button
									variant="solid"
									tone="brand"
									onClick={onSubmit}
									disabled={isLoading}
								>
									{isLoading ? (
										<>
											<Spinner />
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
							</Box>
						</Box>
					</Box>
				</Box>
			</Box>
		</TabsContent>
	);
}
