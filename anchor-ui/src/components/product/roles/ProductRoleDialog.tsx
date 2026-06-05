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
import { type ReactNode, useEffect, useState } from "react";
import { toast } from "sonner";
import type { BasicInfoFormData, RoleFormData } from "./form-type";
import { BasicInfoStep, ReviewStep } from "./steps";

interface ProductRoleDialogProps {
	productId: string;
	trigger: ReactNode;
	onSaved?: () => void;
	mode?: "create" | "edit";
	existingRole?: ProductRoleResponse;
}

const steps: Step[] = [
	{ id: "basic", title: "Basic Info", icon: User },
	{ id: "permissions", title: "Permissions", icon: Shield },
	{ id: "review", title: "Review", icon: ClipboardCheck },
];

export function ProductRoleDialog({
	productId,
	trigger,
	onSaved,
	mode = "create",
	existingRole,
}: ProductRoleDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [currentStep, setCurrentStep] = useState(0);
	const [formData, setFormData] = useState<RoleFormData>({
		name: "",
		description: "",
		selectedPermissions: [],
	});

	const isEditMode = mode === "edit";

	useEffect(() => {
		if (open) {
			if (isEditMode && existingRole) {
				setFormData({
					name: existingRole.name,
					description: existingRole.description || "",
					selectedPermissions:
						existingRole.permissions?.map((perm) => perm.permission_name) || [],
				});
			} else {
				setFormData({
					name: "",
					description: "",
					selectedPermissions: [],
				});
			}
			setCurrentStep(0);
		}
	}, [open, isEditMode, existingRole]);

	const handleSuccess = () => {
		setOpen(false);
		resetForm();

		queryClient.invalidateQueries({
			queryKey: searchProductRolesQueryKey({
				path: { product_id: productId },
				body: {},
			}),
		});

		onSaved?.();
	};

	const handleError = (error: unknown) => {
		console.error(
			`Failed to ${isEditMode ? "update" : "create"} product role:`,
			error,
		);
		const errorMessage = getApiErrorMessage(error);
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

	const resetForm = () => {
		setFormData({ name: "", description: "", selectedPermissions: [] });
		setCurrentStep(0);
	};

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
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>{trigger}</DialogTrigger>
			<DialogContent className="p-0 sm:max-w-[900px] max-h-[95vh] overflow-hidden">
				<DialogHeader className="sr-only">
					<DialogTitle>{isEditMode ? "Edit Role" : "Create Role"}</DialogTitle>
					<DialogDescription>
						{isEditMode
							? "Update the details and permissions for this role."
							: "Create a new role by providing its details and permissions."}
					</DialogDescription>
				</DialogHeader>
				<div className="flex h-[700px]">
					<VerticalStepper
						steps={steps}
						currentStep={currentStep}
						onStepChange={setCurrentStep}
						title={isEditMode ? "Edit Role" : "Create Role"}
						titleIcon={isEditMode ? Edit : Sparkles}
						showProgress={true}
						allowStepNavigation={false}
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
							onCancel={() => setOpen(false)}
						/>
					</VerticalStepper>
				</div>
			</DialogContent>
		</Dialog>
	);
}
