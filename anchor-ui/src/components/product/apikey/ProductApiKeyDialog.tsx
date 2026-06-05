import type {
	CreatedProductApiKeyResponse,
	ProductApiKeyCreateRequest,
	ProductApiKeyResponse,
	ProductApiKeyUpdateRequest,
} from "@/client";
import {
	createProductApiKeyMutation,
	searchProductApiKeysQueryKey,
	updateProductApiKeyMutation,
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
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
	CheckCircle,
	ClipboardCheck,
	Edit,
	Shield,
	Sparkles,
	User,
} from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";
import { toast } from "sonner";
import type { ApiKeyFormData, BasicInfoFormData } from "./form-type";
import { BasicInfoStep, ReviewStep, SuccessStep } from "./steps";

interface ProductApiKeyDialogProps {
	productId: string;
	trigger: ReactNode;
	onSaved?: () => void;
	mode?: "create" | "edit";
	existingApiKey?: ProductApiKeyResponse;
}

const createSteps: Step[] = [
	{ id: "basic", title: "Basic Info", icon: User },
	{ id: "permissions", title: "Permissions", icon: Shield },
	{ id: "review", title: "Review", icon: ClipboardCheck },
	{ id: "success", title: "Success", icon: CheckCircle },
];

interface MutationErrorShape {
	errors?: Array<{
		message?: string;
	}>;
}

const editSteps = (includePermissions: boolean): Step[] => {
	if (includePermissions) {
		return [
			{ id: "basic", title: "Basic Info", icon: User },
			{ id: "permissions", title: "Permissions", icon: Shield },
			{ id: "review", title: "Review", icon: ClipboardCheck },
		];
	}

	return [
		{ id: "basic", title: "Basic Info", icon: User },
		{ id: "review", title: "Review", icon: ClipboardCheck },
	];
};

export function ProductApiKeyDialog({
	productId,
	trigger,
	onSaved,
	mode = "create",
	existingApiKey,
}: ProductApiKeyDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [currentStep, setCurrentStep] = useState(0);
	const [formData, setFormData] = useState<ApiKeyFormData>({
		name: "",
		description: "",
		mutable: false,
		selectedPermissions: [],
	});
	const [createdApiKey, setCreatedApiKey] =
		useState<CreatedProductApiKeyResponse | null>(null);

	const isEditMode = mode === "edit";
	const canEditPermissions = isEditMode && Boolean(existingApiKey?.mutable);
	const steps = isEditMode ? editSteps(canEditPermissions) : createSteps;

	useEffect(() => {
		if (open) {
			if (isEditMode && existingApiKey) {
				setFormData({
					name: existingApiKey.name,
					description: existingApiKey.description || "",
					mutable: existingApiKey.mutable,
					selectedPermissions:
						existingApiKey.permissions?.map((perm) => perm.permission_name) ||
						[],
				});
			} else {
				setFormData({
					name: "",
					description: "",
					mutable: false,
					selectedPermissions: [],
				});
			}
			setCurrentStep(0);
			setCreatedApiKey(null);
		}
	}, [open, isEditMode, existingApiKey]);

	const handleSuccess = () => {
		if (isEditMode) {
			setOpen(false);
			resetForm();
			queryClient.invalidateQueries({
				queryKey: searchProductApiKeysQueryKey({
					path: { product_id: productId },
					body: {},
				}),
			});
			onSaved?.();
		}
	};

	const handleCreateSuccess = (response: CreatedProductApiKeyResponse) => {
		setCreatedApiKey(response);
		setCurrentStep(steps.length - 1); // Go to success step
		toast.success("API Key created successfully!", {
			description: `${formData.name} is ready to use`,
		});
	};

	const handleFinalClose = () => {
		setOpen(false);
		resetForm();
		queryClient.invalidateQueries({
			queryKey: searchProductApiKeysQueryKey({
				path: { product_id: productId },
				body: {},
			}),
		});
		onSaved?.();
	};

	const handleError = (error: unknown) => {
		console.error(
			`Failed to ${isEditMode ? "update" : "create"} API key:`,
			error,
		);
		const apiError = error as MutationErrorShape;
		if (apiError.errors?.[0]?.message) {
			toast.error(apiError.errors[0].message);
		} else {
			toast.error(
				`Failed to ${isEditMode ? "update" : "create"} API key. Please try again.`,
			);
		}
	};

	const createMutation = useMutation({
		...createProductApiKeyMutation(),
		onSuccess: handleCreateSuccess,
		onError: handleError,
	});

	const updateMutation = useMutation({
		...updateProductApiKeyMutation(),
		onSuccess: () => {
			toast.success("API Key updated successfully!", {
				description: `${formData.name} has been updated`,
			});
			handleSuccess();
		},
		onError: handleError,
	});

	const resetForm = () => {
		setFormData({
			name: "",
			description: "",
			mutable: false,
			selectedPermissions: [],
		});
		setCurrentStep(0);
		setCreatedApiKey(null);
	};

	const nextStep = () => {
		setCurrentStep((prev) => Math.min(prev + 1, steps.length - 1));
	};

	const prevStep = () => {
		setCurrentStep((prev) => Math.max(prev - 1, 0));
	};

	const handleSubmit = async () => {
		if (isEditMode && existingApiKey) {
			const updateData: ProductApiKeyUpdateRequest = {
				name: formData.name,
				description: formData.description || undefined,
				permissions: canEditPermissions
					? formData.selectedPermissions
					: undefined,
			};

			updateMutation.mutate({
				path: {
					product_id: productId,
					api_key_id: existingApiKey.id,
				},
				body: updateData,
			});
		} else {
			const createData: ProductApiKeyCreateRequest = {
				name: formData.name,
				description: formData.description || undefined,
				mutable: formData.mutable,
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
					<DialogTitle>
						{isEditMode ? "Edit API Key" : "Create API Key"}
					</DialogTitle>
					<DialogDescription>
						{isEditMode
							? "Update the details and permissions for this API key."
							: "Create a new API key by providing its details and permissions."}
					</DialogDescription>
				</DialogHeader>
				<div className="flex h-[700px]">
					<VerticalStepper
						steps={steps}
						currentStep={currentStep}
						onStepChange={setCurrentStep}
						title={isEditMode ? "Edit API Key" : "Create API Key"}
						titleIcon={isEditMode ? Edit : Sparkles}
						showProgress={true}
						allowStepNavigation={isEditMode || currentStep !== 3}
					>
						<BasicInfoStep
							initialData={{
								name: formData.name,
								description: formData.description,
								mutable: formData.mutable,
							}}
							onFormDataChange={(data: BasicInfoFormData) => {
								setFormData((prev) => ({
									...prev,
									name: data.name,
									description: data.description ?? "",
									mutable: data.mutable,
								}));
							}}
							isEditMode={isEditMode}
							apiKey={existingApiKey}
							onNext={nextStep}
							onCancel={() => setOpen(false)}
						/>

						{(!isEditMode || canEditPermissions) && (
							<CommonPermissionsStep
								productId={productId}
								variant={"product"}
								initialSelectedPermissions={
									isEditMode
										? (existingApiKey?.permissions?.map(
												(permission) => permission.permission_name,
											) ?? [])
										: undefined
								}
								onFormDataChange={(data: { selectedPermissions: string[] }) => {
									setFormData((prev) => ({
										...prev,
										selectedPermissions: data.selectedPermissions,
									}));
								}}
								isEditMode={isEditMode}
								existingItem={existingApiKey}
								onNext={nextStep}
								onPrevious={prevStep}
								config={{
									description: "Select permissions to assign to this API key",
									itemType: "API key",
									showSelectedPermissionsScrollArea: true,
									showNavigateToPermissions: true,
								}}
							/>
						)}

						<ReviewStep
							apiKeyFormData={formData}
							isEditMode={isEditMode}
							apiKey={existingApiKey}
							isLoading={isLoading}
							onSubmit={handleSubmit}
							onPrevious={prevStep}
						/>

						{!isEditMode && createdApiKey && (
							<SuccessStep
								createdApiKey={createdApiKey}
								onClose={handleFinalClose}
							/>
						)}
					</VerticalStepper>
				</div>
			</DialogContent>
		</Dialog>
	);
}
