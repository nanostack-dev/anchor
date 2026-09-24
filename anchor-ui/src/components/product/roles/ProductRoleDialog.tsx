import type {
	ProductRoleCreateRequest,
	ProductRoleResponse,
	ProductRoleUpdateRequest,
} from "@/client";
import {
	createProductRoleMutation,
	searchProductRolesQueryKey,
	updateProductRoleMutation,
} from "@/client/@tanstack/react-query.gen";
import { PermissionsStep as CommonPermissionsStep } from "@/components/product/common/steps/PermissionsStep";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { type Step, VerticalStepper } from "@/components/ui/vertical-stepper";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { ClipboardCheck, Edit, Shield, Sparkles, User } from "lucide-react";
import { type ReactElement, useState } from "react";
import { toast } from "sonner";
import type { BasicInfoFormData, RoleFormData } from "./form-type";
import { BasicInfoStep, ReviewStep } from "./steps";

interface ProductRoleDialogProps {
	productId: string;
	trigger: ReactElement;
	onSaved?: () => void;
	mode?: "create" | "edit";
	existingRole?: ProductRoleResponse;
}

interface ProductRoleEditorProps {
	productId: string;
	mode: "create" | "edit";
	existingRole?: ProductRoleResponse;
	onSaved: () => void;
	onCancel: () => void;
}

const steps: Step[] = [
	{ id: "basic", title: "Basic Info", icon: User },
	{ id: "permissions", title: "Permissions", icon: Shield },
	{ id: "review", title: "Review", icon: ClipboardCheck },
];

export function ProductRoleEditor({
	productId,
	mode,
	existingRole,
	onSaved,
	onCancel,
}: ProductRoleEditorProps) {
	const queryClient = useQueryClient();
	const [currentStep, setCurrentStep] = useState(0);
	const [saveError, setSaveError] = useState<string | null>(null);
	const [formData, setFormData] = useState<RoleFormData>(() => ({
		name: existingRole?.name ?? "",
		description: existingRole?.description ?? "",
		selectedPermissions:
			existingRole?.permissions?.map((perm) => perm.permission_name) ?? [],
	}));

	const isEditMode = mode === "edit";

	const handleSuccess = () => {
		queryClient.invalidateQueries({
			queryKey: searchProductRolesQueryKey({
				path: { product_id: productId },
				body: {},
			}),
		});

		onSaved();
	};

	const handleError = (error: unknown) => {
		console.error(
			`Failed to ${isEditMode ? "update" : "create"} product role:`,
			error,
		);
		const errorMessage = getApiErrorMessage(error);
		setSaveError(
			errorMessage ||
				`Failed to ${isEditMode ? "update" : "create"} role. Please try again.`,
		);
		if (errorMessage) {
			toast.error(errorMessage);
		} else {
			toast.error(
				`Failed to ${isEditMode ? "update" : "create"} role. Please try again.`,
			);
		}
	};

	const createMutation = useMutation({
		...createProductRoleMutation(),
		onSuccess: () => {
			toast.success("Role created successfully!", {
				description: `${formData.name} is ready to use`,
			});
			handleSuccess();
		},
		onError: handleError,
	});

	const updateMutation = useMutation({
		...updateProductRoleMutation(),
		onSuccess: () => {
			toast.success("Role updated successfully!", {
				description: `${formData.name} has been updated`,
			});
			handleSuccess();
		},
		onError: handleError,
	});

	const nextStep = () => {
		setCurrentStep((prev) => Math.min(prev + 1, steps.length - 1));
	};

	const prevStep = () => {
		setCurrentStep((prev) => Math.max(prev - 1, 0));
	};

	const handleSubmit = async () => {
		if (isEditMode && existingRole) {
			const updateData: ProductRoleUpdateRequest = {
				name: formData.name,
				description: formData.description || undefined,
				permissions: formData.selectedPermissions,
			};

			updateMutation.mutate({
				path: {
					product_id: productId,
					role_id: existingRole.id,
				},
				body: updateData,
			});
		} else {
			const createData: ProductRoleCreateRequest = {
				name: formData.name,
				description: formData.description || undefined,
				permissions:
					formData.selectedPermissions.length > 0
						? formData.selectedPermissions
						: undefined,
			};

			createMutation.mutate({
				path: { product_id: productId },
				body: createData,
			});
		}
	};

	const isLoading = createMutation.isPending || updateMutation.isPending;

	return (
		<div className="space-y-3">
			{saveError && (
				<p role="alert" className="text-sm text-destructive">
					{saveError}
				</p>
			)}
			<div className="flex h-[700px] overflow-hidden rounded-lg border border-border">
				<VerticalStepper
					steps={steps}
					currentStep={currentStep}
					onStepChange={setCurrentStep}
					title={isEditMode ? "Edit Role" : "Create Role"}
					titleIcon={isEditMode ? Edit : Sparkles}
					showProgress={true}
					allowStepNavigation={false}
					sidebarClassName="hidden md:block"
				>
					<BasicInfoStep
						initialData={{
							name: formData.name,
							description: formData.description,
						}}
						onFormDataChange={(data: BasicInfoFormData) => {
							setFormData((prev) => {
								return {
									...prev,
									name: data.name,
									description: data.description,
								};
							});
							nextStep();
						}}
						isEditMode={isEditMode}
						existingRole={existingRole}
					/>

					<CommonPermissionsStep
						variant={"resource"}
						productId={productId}
						onFormDataChange={(data: { selectedPermissions: string[] }) => {
							setFormData((prev) => ({
								...prev,
								selectedPermissions: data.selectedPermissions,
							}));
						}}
						isEditMode={isEditMode}
						existingItem={existingRole}
						initialSelectedPermissions={existingRole?.permissions.map(
							(permission) => permission.permission_name,
						)}
						onNext={nextStep}
						onPrevious={prevStep}
						config={{
							description: "Select permissions to assign to this role",
							itemType: "role",
							showSelectedPermissionsScrollArea: true,
							showNavigateToPermissions: true,
						}}
					/>

					<ReviewStep
						formData={formData}
						isEditMode={isEditMode}
						existingRole={existingRole}
						isLoading={isLoading}
						onSubmit={handleSubmit}
						onPrevious={prevStep}
						onCancel={onCancel}
					/>
				</VerticalStepper>
			</div>
		</div>
	);
}

export function ProductRoleDialog({
	productId,
	trigger,
	onSaved,
	mode = "create",
	existingRole,
}: ProductRoleDialogProps) {
	const [open, setOpen] = useState(false);
	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger} />
			<DialogContent className="p-0 sm:max-w-[900px] max-h-[95vh] overflow-hidden">
				<DialogHeader className="sr-only">
					<DialogTitle>
						{mode === "edit" ? "Edit Role" : "Create Role"}
					</DialogTitle>
					<DialogDescription>
						{mode === "edit"
							? "Update the details and permissions for this role."
							: "Create a new role by providing its details and permissions."}
					</DialogDescription>
				</DialogHeader>
				{open && (
					<ProductRoleEditor
						productId={productId}
						mode={mode}
						existingRole={existingRole}
						onSaved={() => {
							setOpen(false);
							onSaved?.();
						}}
						onCancel={() => setOpen(false)}
					/>
				)}
			</DialogContent>
		</Dialog>
	);
}
