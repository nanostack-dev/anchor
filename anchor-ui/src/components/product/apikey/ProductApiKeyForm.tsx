import type {
	CreatedProductApiKeyResponse,
	ProductApiKeyCreateRequest,
	ProductApiKeyUpdateRequest,
} from "@/client";
import {
	createProductApiKeyMutation,
	getProductApiKeyOptions,
	updateProductApiKeyMutation,
} from "@/client/@tanstack/react-query.gen";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import {
	AlertCircle,
	ArrowLeft,
	Check,
	CheckCircle,
	ChevronLeft,
	ChevronRight,
	ClipboardCheck,
	Copy,
	Edit,
	Eye,
	EyeOff,
	Key,
	Lock,
	Plus,
	Shield,
} from "lucide-react";
import { type ComponentType, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { ApiKeyPermissionSelector } from "./ApiKeyPermissionSelector";
import { basicInfo } from "./form-type";

interface ProductApiKeyFormProps {
	productId: string;
	productName: string;
	mode: "create" | "edit";
	apiKeyId?: string;
}

type StepId = "details" | "permissions" | "review";

interface StepDef {
	id: StepId;
	title: string;
	icon: ComponentType<{ className?: string }>;
}

interface MutationErrorShape {
	errors?: Array<{ message?: string }>;
}

const NAME_MIN = 2;
const NAME_MAX = 100;
const DESCRIPTION_MAX = 500;

function getErrorMessage(error: unknown, fallback: string): string {
	const apiError = error as MutationErrorShape;
	return apiError?.errors?.[0]?.message ?? fallback;
}

export function ProductApiKeyForm({
	productId,
	productName,
	mode,
	apiKeyId,
}: ProductApiKeyFormProps) {
	const navigate = useNavigate();
	const queryClient = useQueryClient();
	const isEditMode = mode === "edit";

	const {
		data: existingApiKey,
		isLoading: isLoadingExisting,
		isError: existingError,
	} = useQuery({
		...getProductApiKeyOptions({
			path: { product_id: productId, api_key_id: apiKeyId ?? "" },
		}),
		enabled: isEditMode && Boolean(apiKeyId),
	});

	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [mutable, setMutable] = useState(false);
	const [permissions, setPermissions] = useState<string[]>([]);
	const [nameTouched, setNameTouched] = useState(false);
	const [currentStep, setCurrentStep] = useState(0);
	const [createdApiKey, setCreatedApiKey] =
		useState<CreatedProductApiKeyResponse | null>(null);

	const seeded = useRef(false);
	useEffect(() => {
		if (isEditMode && existingApiKey && !seeded.current) {
			setName(existingApiKey.name);
			setDescription(existingApiKey.description ?? "");
			setMutable(existingApiKey.mutable);
			setPermissions(
				existingApiKey.permissions?.map((p) => p.permission_name) ?? [],
			);
			seeded.current = true;
		}
	}, [isEditMode, existingApiKey]);

	const canEditPermissions = isEditMode && Boolean(existingApiKey?.mutable);
	const includePermissionsStep = !isEditMode || canEditPermissions;

	const steps: StepDef[] = [
		{ id: "details", title: "Details", icon: Key },
		...(includePermissionsStep
			? [{ id: "permissions" as const, title: "Permissions", icon: Shield }]
			: []),
		{ id: "review", title: "Review", icon: ClipboardCheck },
	];

	const activeStep = steps[currentStep]?.id;

	const invalidateList = () => {
		void queryClient.invalidateQueries({
			predicate: (query) =>
				(query.queryKey[0] as { _id?: string } | undefined)?._id ===
				"searchProductApiKeys",
		});
	};

	const createMutation = useMutation({
		...createProductApiKeyMutation(),
		onSuccess: (response: CreatedProductApiKeyResponse) => {
			setCreatedApiKey(response);
			invalidateList();
			toast.success("API key created", { description: `${name} is ready` });
		},
		onError: (error) =>
			toast.error(getErrorMessage(error, "Failed to create API key")),
	});

	const updateMutation = useMutation({
		...updateProductApiKeyMutation(),
		onSuccess: () => {
			invalidateList();
			toast.success("API key updated", { description: `${name} was updated` });
			void navigate({ to: ROUTE_PATHS.PRODUCT_API_KEYS });
		},
		onError: (error) =>
			toast.error(getErrorMessage(error, "Failed to update API key")),
	});

	const isSubmitting = createMutation.isPending || updateMutation.isPending;

	// --- Validation -----------------------------------------------------------
	const detailsResult = basicInfo.safeParse({
		name,
		description: description || undefined,
		mutable,
	});
	const nameError =
		name.trim().length < NAME_MIN
			? `Name must be at least ${NAME_MIN} characters`
			: name.length > NAME_MAX
				? `Name must be less than ${NAME_MAX} characters`
				: null;
	const detailsValid = detailsResult.success;
	const permissionsValid = permissions.length > 0;

	const canAdvance =
		activeStep === "details"
			? detailsValid
			: activeStep === "permissions"
				? permissionsValid
				: true;

	const goBackToList = () =>
		void navigate({ to: ROUTE_PATHS.PRODUCT_API_KEYS });

	const handleNext = () => {
		if (activeStep === "details") setNameTouched(true);
		if (!canAdvance) return;
		setCurrentStep((s) => Math.min(s + 1, steps.length - 1));
	};

	const handleBack = () => {
		if (currentStep === 0) {
			goBackToList();
			return;
		}
		setCurrentStep((s) => Math.max(s - 1, 0));
	};

	const handleSubmit = () => {
		if (isEditMode && existingApiKey) {
			const body: ProductApiKeyUpdateRequest = {
				name,
				description: description || undefined,
				permissions: canEditPermissions ? permissions : undefined,
			};
			updateMutation.mutate({
				path: { product_id: productId, api_key_id: existingApiKey.id },
				body,
			});
		} else {
			const body: ProductApiKeyCreateRequest = {
				name,
				description: description || undefined,
				mutable,
				permissions: permissions.length > 0 ? permissions : undefined,
			};
			createMutation.mutate({ path: { product_id: productId }, body });
		}
	};

	// --- Render branches ------------------------------------------------------
	if (isEditMode && isLoadingExisting) {
		return (
			<div className="flex flex-col items-center justify-center h-64 gap-3">
				<Spinner className="size-7 text-current" />
				<p className="text-sm text-muted-foreground">Loading API key…</p>
			</div>
		);
	}

	if (isEditMode && (existingError || !existingApiKey)) {
		return (
			<div className="flex flex-col items-center justify-center h-64 gap-3 text-center">
				<AlertCircle className="size-8 text-destructive" />
				<p className="text-sm text-muted-foreground">
					This API key could not be found.
				</p>
				<Button variant="outline" onClick={goBackToList}>
					<ArrowLeft className="mr-2 size-4" />
					Back to API Keys
				</Button>
			</div>
		);
	}

	if (createdApiKey) {
		return <SuccessPanel createdApiKey={createdApiKey} onDone={goBackToList} />;
	}

	return (
		<div className="flex flex-col gap-6">
			{/* Header */}
			<div className="flex flex-col gap-2">
				<Link
					to={ROUTE_PATHS.PRODUCT_API_KEYS}
					className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground w-fit"
				>
					<ArrowLeft className="size-4" />
					API Keys
				</Link>
				<div className="flex flex-wrap items-center gap-2">
					<h1 className="text-2xl font-semibold tracking-tight">
						{isEditMode ? "Edit API Key" : "Create API Key"}
					</h1>
					{isEditMode && existingApiKey && (
						<Badge variant="outline" className="font-mono">
							{existingApiKey.name}
						</Badge>
					)}
				</div>
				<p className="text-sm text-muted-foreground">
					{isEditMode
						? `Update the details and permissions for this API key in ${productName}.`
						: `Create a new API key to grant programmatic access to ${productName}.`}
				</p>
			</div>

			<Stepper steps={steps} current={currentStep} />

			{/* Step content */}
			<div className="rounded-xl border border-border bg-card p-4 sm:p-6">
				{activeStep === "details" && (
					<div className="flex flex-col gap-6">
						<div className="flex flex-col gap-2">
							<Label htmlFor="name" className="text-sm font-semibold">
								Name <span className="text-destructive">*</span>
							</Label>
							<Input
								id="name"
								value={name}
								onChange={(e) => setName(e.target.value)}
								onBlur={() => setNameTouched(true)}
								placeholder="e.g. Production Key, Analytics API"
								className="h-11"
							/>
							{nameTouched && nameError && (
								<p className="text-xs text-destructive">{nameError}</p>
							)}
						</div>

						<div className="flex flex-col gap-2">
							<Label htmlFor="description" className="text-sm font-semibold">
								Description
							</Label>
							<Textarea
								id="description"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Describe what this API key is used for…"
								rows={4}
								className="resize-none"
							/>
							<p
								className={cn(
									"text-xs",
									description.length > DESCRIPTION_MAX
										? "text-destructive"
										: "text-muted-foreground",
								)}
							>
								{description.length}/{DESCRIPTION_MAX}
							</p>
						</div>

						{!isEditMode && (
							<div className="flex items-start justify-between gap-4 rounded-lg border border-border p-4">
								<div className="space-y-1">
									<Label htmlFor="mutable" className="text-sm font-semibold">
										Mutable permissions
									</Label>
									<p className="text-xs text-muted-foreground">
										Allow this key's permissions to be changed after creation.
										This can't be toggled later.
									</p>
								</div>
								<Switch
									id="mutable"
									checked={mutable}
									onCheckedChange={setMutable}
								/>
							</div>
						)}

						{isEditMode && !canEditPermissions && (
							<div className="flex items-start gap-3 rounded-lg border border-border bg-muted/50 p-4">
								<Lock className="size-4 mt-0.5 text-muted-foreground shrink-0" />
								<p className="text-xs text-muted-foreground">
									This API key was created as immutable, so its permissions
									cannot be changed. You can still update its name and
									description.
								</p>
							</div>
						)}
					</div>
				)}

				{activeStep === "permissions" && (
					<div className="flex flex-col gap-4">
						<div>
							<h2 className="text-sm font-semibold">Permissions</h2>
							<p className="text-xs text-muted-foreground mt-0.5">
								Select at least one permission to grant this API key.
							</p>
						</div>
						<ApiKeyPermissionSelector
							productId={productId}
							value={permissions}
							onChange={setPermissions}
							originalPermissions={
								existingApiKey?.permissions?.map((p) => p.permission_name) ?? []
							}
						/>
					</div>
				)}

				{activeStep === "review" && (
					<ReviewContent
						name={name}
						description={description}
						mutable={isEditMode ? (existingApiKey?.mutable ?? false) : mutable}
						permissions={
							isEditMode && !canEditPermissions
								? (existingApiKey?.permissions?.map((p) => p.permission_name) ??
									[])
								: permissions
						}
						isEditMode={isEditMode}
						permissionsEditable={!isEditMode || canEditPermissions}
					/>
				)}
			</div>

			{/* Footer actions */}
			<div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
				<Button
					type="button"
					variant="outline"
					onClick={handleBack}
					className="w-full sm:w-auto"
				>
					{currentStep === 0 ? (
						"Cancel"
					) : (
						<>
							<ChevronLeft className="mr-1 size-4" />
							Back
						</>
					)}
				</Button>

				{activeStep === "review" ? (
					<Button
						type="button"
						onClick={handleSubmit}
						disabled={isSubmitting}
						className="w-full sm:w-auto"
					>
						{isSubmitting ? (
							<>
								<Spinner data-icon="inline-start" className="text-current" />
								{isEditMode ? "Updating…" : "Creating…"}
							</>
						) : (
							<>
								{isEditMode ? (
									<Edit className="mr-1 size-4" />
								) : (
									<Plus className="mr-1 size-4" />
								)}
								{isEditMode ? "Update API Key" : "Create API Key"}
							</>
						)}
					</Button>
				) : (
					<Button
						type="button"
						onClick={handleNext}
						disabled={!canAdvance}
						className="w-full sm:w-auto"
					>
						Next
						<ChevronRight className="ml-1 size-4" />
					</Button>
				)}
			</div>
		</div>
	);
}

// ---------------------------------------------------------------------------
// Stepper — horizontal, responsive (compact on mobile, full pills on desktop)
// ---------------------------------------------------------------------------
function Stepper({ steps, current }: { steps: StepDef[]; current: number }) {
	const progress = ((current + 1) / steps.length) * 100;

	return (
		<div>
			{/* Mobile */}
			<div className="flex items-center gap-3 sm:hidden">
				<div className="flex size-9 shrink-0 items-center justify-center rounded-full bg-primary text-primary-foreground text-sm font-semibold">
					{current + 1}
				</div>
				<div className="min-w-0 flex-1">
					<p className="text-xs text-muted-foreground">
						Step {current + 1} of {steps.length}
					</p>
					<p className="text-sm font-medium truncate">
						{steps[current]?.title}
					</p>
				</div>
				<div className="h-1.5 w-16 shrink-0 rounded-full bg-muted overflow-hidden">
					<div
						className="h-full bg-primary transition-all duration-300"
						style={{ width: `${progress}%` }}
					/>
				</div>
			</div>

			{/* Desktop */}
			<ol className="hidden sm:flex items-center">
				{steps.map((step, i) => {
					const state =
						i < current ? "completed" : i === current ? "active" : "pending";
					const Icon = step.icon;
					return (
						<li key={step.id} className="flex items-center">
							<div className="flex items-center gap-2">
								<div
									className={cn(
										"flex size-8 items-center justify-center rounded-full border transition-colors",
										state === "active" &&
											"bg-primary text-primary-foreground border-primary",
										state === "completed" &&
											"bg-success text-success-foreground border-success",
										state === "pending" &&
											"bg-muted text-muted-foreground border-border",
									)}
								>
									{state === "completed" ? (
										<Check className="size-4" />
									) : (
										<Icon className="size-4" />
									)}
								</div>
								<span
									className={cn(
										"text-sm font-medium",
										state === "active"
											? "text-foreground"
											: state === "completed"
												? "text-success"
												: "text-muted-foreground",
									)}
								>
									{step.title}
								</span>
							</div>
							{i < steps.length - 1 && (
								<div className="mx-3 h-px w-10 bg-border" />
							)}
						</li>
					);
				})}
			</ol>
		</div>
	);
}

// ---------------------------------------------------------------------------
// Review
// ---------------------------------------------------------------------------
function ReviewContent({
	name,
	description,
	mutable,
	permissions,
	isEditMode,
	permissionsEditable,
}: {
	name: string;
	description: string;
	mutable: boolean;
	permissions: string[];
	isEditMode: boolean;
	permissionsEditable: boolean;
}) {
	return (
		<div className="flex flex-col gap-5">
			<div className="flex items-center gap-3 rounded-xl border border-border bg-success/10 p-4">
				<CheckCircle className="size-5 text-success shrink-0" />
				<p className="text-sm text-success">
					Review the configuration below, then{" "}
					{isEditMode ? "update" : "create"} the API key.
				</p>
			</div>

			<div className="grid gap-4 sm:grid-cols-2">
				<div className="rounded-lg bg-muted/50 p-4">
					<p className="text-xs font-semibold text-muted-foreground">Name</p>
					<p className="text-base font-medium mt-1 break-words">{name}</p>
				</div>
				<div className="rounded-lg bg-muted/50 p-4">
					<p className="text-xs font-semibold text-muted-foreground">
						Permissions mutability
					</p>
					<p className="text-base font-medium mt-1">
						{mutable ? "Mutable" : "Immutable"}
					</p>
				</div>
			</div>

			{description && (
				<div className="rounded-lg bg-muted/50 p-4">
					<p className="text-xs font-semibold text-muted-foreground">
						Description
					</p>
					<p className="text-sm mt-1 whitespace-pre-wrap break-words">
						{description}
					</p>
				</div>
			)}

			<div className="rounded-lg bg-muted/50 p-4">
				<p className="text-xs font-semibold text-muted-foreground">
					Permissions ({permissions.length})
				</p>
				{permissions.length === 0 ? (
					<p className="text-sm text-muted-foreground mt-1">
						No permissions assigned
					</p>
				) : (
					<div className="flex flex-wrap gap-2 mt-2">
						{permissions.map((permission) => (
							<Badge
								key={permission}
								variant="outline"
								className="text-xs font-mono"
							>
								{permission}
							</Badge>
						))}
					</div>
				)}
				{isEditMode && !permissionsEditable && (
					<p className="text-xs text-muted-foreground mt-2">
						Permissions are immutable and cannot be changed.
					</p>
				)}
			</div>

			{!isEditMode && (
				<div className="flex items-start gap-3 rounded-xl border border-border bg-warning/10 p-4">
					<Lock className="size-5 text-warning shrink-0 mt-0.5" />
					<p className="text-sm text-warning">
						The API key value is shown only once, right after creation. Make
						sure to copy and store it securely.
					</p>
				</div>
			)}
		</div>
	);
}

// ---------------------------------------------------------------------------
// Success panel — shown after a key is created
// ---------------------------------------------------------------------------
function SuccessPanel({
	createdApiKey,
	onDone,
}: {
	createdApiKey: CreatedProductApiKeyResponse;
	onDone: () => void;
}) {
	const [revealed, setRevealed] = useState(false);
	const [copied, setCopied] = useState(false);

	const copy = async () => {
		if (!createdApiKey.value) return;
		try {
			await navigator.clipboard.writeText(createdApiKey.value);
			setCopied(true);
			toast.success("API key copied to clipboard");
			setTimeout(() => setCopied(false), 2000);
		} catch {
			toast.error("Failed to copy API key");
		}
	};

	return (
		<div className="flex flex-col gap-6">
			<div className="flex flex-col gap-2">
				<div className="flex items-center gap-2">
					<CheckCircle className="size-6 text-success" />
					<h1 className="text-2xl font-semibold tracking-tight">
						API Key Created
					</h1>
				</div>
				<p className="text-sm text-muted-foreground">
					<span className="font-medium text-foreground">
						{createdApiKey.name}
					</span>{" "}
					is ready to use.
				</p>
			</div>

			<div className="rounded-xl border border-border bg-warning/10 p-4 sm:p-6 flex flex-col gap-4">
				<div className="flex items-start gap-3">
					<AlertCircle className="size-5 text-warning shrink-0 mt-0.5" />
					<div>
						<p className="text-sm font-semibold text-warning">
							Copy your API key now
						</p>
						<p className="text-sm text-warning mt-1">
							This is the only time the full value is shown. Store it somewhere
							secure.
						</p>
					</div>
				</div>

				<div className="flex flex-col gap-2 sm:flex-row sm:items-center">
					<div className="flex-1 min-w-0 rounded-lg border border-border bg-muted p-3 font-mono text-sm break-all">
						{revealed ? createdApiKey.value || "No value" : "•".repeat(40)}
					</div>
					<div className="flex gap-2">
						<Button
							type="button"
							variant="outline"
							size="icon"
							className="shrink-0"
							onClick={() => setRevealed((v) => !v)}
							aria-label={revealed ? "Hide value" : "Show value"}
						>
							{revealed ? (
								<EyeOff className="size-4" />
							) : (
								<Eye className="size-4" />
							)}
						</Button>
						<Button
							type="button"
							variant="outline"
							size="icon"
							className="shrink-0"
							onClick={copy}
							aria-label="Copy value"
						>
							{copied ? (
								<Check className="size-4 text-success" />
							) : (
								<Copy className="size-4" />
							)}
						</Button>
					</div>
				</div>
			</div>

			{createdApiKey.permissions && createdApiKey.permissions.length > 0 && (
				<div className="rounded-lg bg-muted/50 p-4">
					<p className="text-xs font-semibold text-muted-foreground">
						Assigned permissions ({createdApiKey.permissions.length})
					</p>
					<div className="flex flex-wrap gap-2 mt-2">
						{createdApiKey.permissions.map((permission) => (
							<Badge
								key={permission.permission_name}
								variant="outline"
								className="text-xs font-mono"
							>
								{permission.permission_name}
							</Badge>
						))}
					</div>
				</div>
			)}

			<div className="flex justify-end">
				<Button type="button" onClick={onDone} className="w-full sm:w-auto">
					<Check className="mr-1 size-4" />
					Done
				</Button>
			</div>
		</div>
	);
}
