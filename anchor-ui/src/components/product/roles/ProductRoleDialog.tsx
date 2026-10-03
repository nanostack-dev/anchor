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
import { getApiErrorMessage } from "@/lib/api-error";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@nanostackorg/design-system/components/dialog";
import {
	Progress,
	ProgressLabel,
} from "@nanostackorg/design-system/components/progress";
import {
	Tabs,
	TabsList,
	TabsTrigger,
} from "@nanostackorg/design-system/components/tabs";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { toast } from "@nanostackorg/design-system/components/toast";
import { type ReactElement, useState } from "react";
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

const steps = [
	{ id: "basic", title: "Basic Info" },
	{ id: "permissions", title: "Permissions" },
	{ id: "review", title: "Review" },
] as const;

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
			toast.add({ type: "error", title: errorMessage });
		} else {
			toast.add({
				type: "error",
				title: `Failed to ${isEditMode ? "update" : "create"} role. Please try again.`,
			});
		}
	};

	const createMutation = useMutation({
		...createProductRoleMutation(),
		onSuccess: () => {
			toast.add({
				type: "success",
				title: "Role created successfully!",
				...{
					description: `${formData.name} is ready to use`,
				},
			});
			handleSuccess();
		},
		onError: handleError,
	});

	const updateMutation = useMutation({
		...updateProductRoleMutation(),
		onSuccess: () => {
			toast.add({
				type: "success",
				title: "Role updated successfully!",
				...{
					description: `${formData.name} has been updated`,
				},
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
		<Box className="space-y-3">
			{saveError && (
				<Box as="p" role="alert" className="text-sm text-destructive">
					{saveError}
				</Box>
			)}
			<Box className="min-w-0">
				<Tabs value={steps[currentStep].id}>
					<Stack space="md">
						<TabsList width="fill" aria-label="Role setup steps">
							{steps.map((step, index) => (
								<TabsTrigger
									key={step.id}
									value={step.id}
									disabled={index !== currentStep}
								>
									{index + 1}. {step.title}
								</TabsTrigger>
							))}
						</TabsList>
						<Progress value={currentStep + 1} max={steps.length}>
							<ProgressLabel>
								Step {currentStep + 1} of {steps.length}
							</ProgressLabel>
						</Progress>
					</Stack>
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
				</Tabs>
			</Box>
		</Box>
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
			<DialogContent size="xl">
				<DialogHeader visuallyHidden>
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
