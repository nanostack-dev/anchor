import type { ProductApiKeyResponse } from "@/client";
import type { ApiKeyFormData } from "@/components/product/apikey/form-type";
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
import { Check, ClipboardCheck, Edit, Lock, Plus } from "lucide-react";

interface ReviewStepProps {
	apiKeyFormData: ApiKeyFormData;
	isEditMode: boolean;
	apiKey?: ProductApiKeyResponse;
	isLoading: boolean;
	onSubmit: () => void;
	onPrevious: () => void;
}

export function ReviewStep({
	apiKeyFormData,
	isEditMode,
	apiKey,
	isLoading,
	onSubmit,
	onPrevious,
}: ReviewStepProps) {
	return (
		<VerticalStepperStep id="review">
			<div className="flex flex-col h-full">
				<div className="px-6 pt-6 pb-4">
					<DialogHeader className="space-y-3">
						<DialogTitle className="flex items-center space-x-3">
							<div className="p-2 rounded-lg bg-primary text-primary-foreground">
								<ClipboardCheck className="h-5 w-5" />
							</div>
							<span className="text-xl">Review</span>
							{isEditMode && (
								<Badge variant="outline" className="ml-2">
									Editing: {apiKey?.name}
								</Badge>
							)}
						</DialogTitle>
						<DialogDescription className="text-base">
							{isEditMode
								? "Review and confirm your API key changes"
								: "Review and confirm your API key configuration"}
						</DialogDescription>
					</DialogHeader>
				</div>

				{/* Scrollable Content */}
				<ScrollArea className="flex-1 px-6">
					<div className="space-y-6 pb-6">
						<div className="p-6 rounded-xl border bg-green-50/50 dark:bg-green-900/10 border-green-200 dark:border-green-800">
							<div className="flex items-center space-x-3 mb-3">
								<div className="p-2 rounded-lg bg-green-100 dark:bg-green-900/30">
									<Check className="h-5 w-5 text-green-600" />
								</div>
								<h4 className="text-lg font-semibold text-green-800 dark:text-green-200">
									Ready to {isEditMode ? "Update" : "Create"} API Key
								</h4>
							</div>
							<p className="text-sm text-green-600 dark:text-green-300">
								Review the configuration below and click{" "}
								{isEditMode ? "update" : "create"} when ready.
							</p>
						</div>

						<div className="space-y-6">
							<div className="p-4 bg-muted/50 rounded-xl">
								<Label className="text-sm font-semibold text-muted-foreground">
									API Key Name
								</Label>
								<p className="text-2xl font-bold text-foreground mt-1">
									{apiKeyFormData.name}
								</p>
							</div>

							{apiKeyFormData.description && (
								<div className="p-4 bg-muted/50 rounded-xl">
									<Label className="text-sm font-semibold text-muted-foreground">
										Description
									</Label>
									<p className="text-base text-foreground mt-1 whitespace-pre-wrap break-words">
										{apiKeyFormData.description}
									</p>
								</div>
							)}

							<div className="p-4 bg-muted/50 rounded-xl">
								<Label className="text-sm font-semibold text-muted-foreground">
									Permissions mutability
								</Label>
								<p className="text-base text-foreground mt-1">
									{apiKeyFormData.mutable ? "Mutable" : "Immutable"}
								</p>
							</div>

							<div className="p-4 bg-muted/50 rounded-xl">
								<Label className="text-sm font-semibold text-muted-foreground">
									Permissions (
									{isEditMode
										? apiKeyFormData.mutable
											? apiKeyFormData.selectedPermissions.length
											: apiKey?.permissions?.length || 0
										: apiKeyFormData.selectedPermissions.length}
									)
								</Label>
								{isEditMode ? (
									(
										apiKeyFormData.mutable
											? apiKeyFormData.selectedPermissions.length > 0
											: apiKey?.permissions && apiKey.permissions.length > 0
									) ? (
										<div className="space-y-3 mt-3">
											<div className="flex flex-wrap gap-2">
												{(apiKeyFormData.mutable
													? apiKeyFormData.selectedPermissions.map(
															(permission) => ({ permission_name: permission }),
														)
													: (apiKey?.permissions ?? [])
												).map((permission) => (
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
												{apiKeyFormData.mutable
													? "Permissions will be updated for this mutable API key"
													: "Permissions are immutable and cannot be changed"}
											</p>
										</div>
									) : (
										<p className="text-base text-muted-foreground mt-1">
											No permissions assigned
										</p>
									)
								) : apiKeyFormData.selectedPermissions.length === 0 ? (
									<p className="text-base text-muted-foreground mt-1">
										No permissions selected
									</p>
								) : (
									<div className="space-y-3 mt-3">
										<div className="flex flex-wrap gap-2">
											{apiKeyFormData.selectedPermissions.map((permission) => (
												<Badge
													key={permission}
													variant="outline"
													className="text-sm px-3 py-1 bg-green-50 border-green-200 text-green-700 dark:bg-green-900/20 dark:border-green-800 dark:text-green-300"
												>
													<code>{permission}</code>
												</Badge>
											))}
										</div>
									</div>
								)}
							</div>
						</div>

						{!isEditMode && apiKeyFormData.selectedPermissions.length > 0 && (
							<div className="p-4 rounded-xl border bg-yellow-50/50 dark:bg-yellow-900/10 border-yellow-200 dark:border-yellow-800">
								<div className="flex items-start space-x-3">
									<div className="p-2 rounded-lg bg-yellow-100 dark:bg-yellow-900/30">
										<Lock className="h-5 w-5 text-yellow-600" />
									</div>
									<div>
										<p className="text-sm font-semibold text-yellow-800 dark:text-yellow-200">
											API Key Value
										</p>
										<p className="text-sm mt-2 text-yellow-600 dark:text-yellow-300">
											The API key value will be shown only once after creation.
											Make sure to copy and store it securely.
										</p>
									</div>
								</div>
							</div>
						)}
					</div>
				</ScrollArea>

				{/* Fixed Footer */}
				<div className="px-6 py-6 border-t mt-auto">
					<DialogFooter className="p-0">
						<div className="flex justify-between w-full">
							<Button
								type="button"
								variant="outline"
								onClick={onPrevious}
								className="px-6"
							>
								Previous
							</Button>

							<div className="flex space-x-3">
								<Button
									onClick={onSubmit}
									disabled={isLoading}
									className="px-8"
								>
									{isLoading ? (
										<>
											<div className="animate-spin rounded-full h-4 w-4 border-b-2 border-white mr-2" />
											{isEditMode ? "Updating..." : "Creating..."}
										</>
									) : (
										<>
											{isEditMode ? (
												<Edit className="mr-2 h-4 w-4" />
											) : (
												<Plus className="mr-2 h-4 w-4" />
											)}
											{isEditMode ? "Update API Key" : "Create API Key"}
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
