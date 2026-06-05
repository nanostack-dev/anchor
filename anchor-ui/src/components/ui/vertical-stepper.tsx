"use client";

import { ScrollArea } from "@/components/ui/scroll-area";
import { Separator } from "@/components/ui/separator";
import { cn } from "@/lib/utils";
import { type VariantProps, cva } from "class-variance-authority";
import { Check } from "lucide-react";
import * as React from "react";

interface Step {
	id: string;
	title: string;
	description?: string;
	icon?: React.ComponentType<{ className?: string }>;
	optional?: boolean;
	disabled?: boolean;
}

interface VerticalStepperContextValue {
	currentStep: number;
	steps: Step[];
	goToStep: (step: number) => void;
	canNavigateToStep: (step: number) => boolean;
	validateStep?: (step: number) => boolean;
	size: "sm" | "default" | "lg" | null;
	allowStepNavigation: boolean;
	showProgress: boolean;
}

const VerticalStepperContext =
	React.createContext<VerticalStepperContextValue | null>(null);

const useVerticalStepperContext = () => {
	const context = React.useContext(VerticalStepperContext);
	if (!context) {
		throw new Error(
			"VerticalStepper components must be used within a VerticalStepper",
		);
	}
	return context;
};

const stepperVariants = cva("flex h-full w-full", {
	variants: {
		size: {
			sm: "text-sm",
			default: "text-base",
			lg: "text-lg",
		},
		variant: {
			default: "",
			compact: "flex-col gap-2",
		},
	},
	defaultVariants: {
		size: "default",
		variant: "default",
	},
});

const stepContentVariants = cva("flex-1 h-full", {
	variants: {
		size: {
			sm: "p-4",
			default: "p-6",
			lg: "p-8",
		},
	},
	defaultVariants: {
		size: "default",
	},
});

const sidebarVariants = cva("bg-muted/60 border-r border-border p-6", {
	variants: {
		size: {
			sm: "w-56 p-4",
			default: "w-64 p-6",
			lg: "w-72 p-8",
		},
	},
	defaultVariants: {
		size: "default",
	},
});

const stepIndicatorVariants = cva(
	"flex items-center justify-center rounded-full transition-all duration-200",
	{
		variants: {
			size: {
				sm: "size-7",
				default: "size-8",
				lg: "size-9",
			},
			state: {
				pending: "bg-muted text-muted-foreground border border-border",
				active: "bg-primary text-primary-foreground shadow-sm",
				completed: "bg-success text-success-foreground",
				disabled:
					"bg-muted text-muted-foreground border border-border opacity-50",
			},
		},
		defaultVariants: {
			size: "default",
			state: "pending",
		},
	},
);

const stepItemVariants = cva(
	"flex items-center gap-3 px-3 py-2.5 rounded-xl transition-all duration-200",
	{
		variants: {
			size: {
				sm: "px-2 py-2 gap-2",
				default: "px-3 py-2.5 gap-3",
				lg: "px-4 py-3 gap-4",
			},
			state: {
				pending: "text-muted-foreground",
				active: "bg-card shadow-sm border border-border text-foreground",
				completed: "text-success",
				disabled: "text-muted-foreground opacity-50",
			},
			interactive: {
				true: "cursor-pointer hover:bg-accent",
				false: "",
			},
		},
		defaultVariants: {
			size: "default",
			state: "pending",
			interactive: false,
		},
	},
);

interface VerticalStepperProps extends VariantProps<typeof stepperVariants> {
	steps: Step[];
	currentStep: number;
	onStepChange?: (step: number) => void;
	onValidateStep?: (step: number) => boolean;
	showProgress?: boolean;
	allowStepNavigation?: boolean;
	title?: string;
	titleIcon?: React.ComponentType<{ className?: string }>;
	className?: string;
	sidebarClassName?: string;
	children?: React.ReactNode;
}

function VerticalStepper({
	steps,
	currentStep,
	onStepChange,
	onValidateStep,
	showProgress = true,
	allowStepNavigation = false,
	title,
	titleIcon: TitleIcon,
	size = "default",
	variant = "default",
	className,
	sidebarClassName,
	children,
	...props
}: VerticalStepperProps) {
	const goToStep = React.useCallback(
		(stepIndex: number) => {
			if (!allowStepNavigation || !onStepChange) return;

			if (
				stepIndex > currentStep &&
				onValidateStep &&
				!onValidateStep(currentStep)
			) {
				return;
			}

			if (stepIndex < currentStep || stepIndex === currentStep + 1) {
				onStepChange(stepIndex);
			}
		},
		[currentStep, onStepChange, onValidateStep, allowStepNavigation],
	);

	const canNavigateToStep = React.useCallback(
		(stepIndex: number) => {
			if (!allowStepNavigation) return false;
			if (steps[stepIndex]?.disabled) return false;
			return stepIndex <= currentStep || stepIndex === currentStep + 1;
		},
		[allowStepNavigation, steps, currentStep],
	);

	const contextValue: VerticalStepperContextValue = {
		currentStep,
		steps,
		goToStep,
		canNavigateToStep,
		validateStep: onValidateStep,
		size,
		allowStepNavigation,
		showProgress,
	};

	return (
		<VerticalStepperContext.Provider value={contextValue}>
			<div
				className={cn(stepperVariants({ size, variant }), className)}
				role="tablist"
				aria-label="Step navigation"
				data-slot="vertical-stepper"
				{...props}
			>
				<VerticalStepperSidebar
					title={title}
					titleIcon={TitleIcon}
					variant={variant || "default"}
					className={sidebarClassName}
				/>
				{children && (
					<VerticalStepperContent>{children}</VerticalStepperContent>
				)}
			</div>
		</VerticalStepperContext.Provider>
	);
}

interface VerticalStepperSidebarProps {
	title?: string;
	titleIcon?: React.ComponentType<{ className?: string }>;
	variant?: "default" | "compact";
	className?: string;
}

function VerticalStepperSidebar({
	title,
	titleIcon: TitleIcon,
	variant = "default",
	className,
}: VerticalStepperSidebarProps) {
	const {
		steps,
		currentStep,
		goToStep,
		canNavigateToStep,
		size,
		showProgress,
	} = useVerticalStepperContext();

	const getStepState = (
		stepIndex: number,
	): "pending" | "active" | "completed" | "disabled" => {
		if (steps[stepIndex]?.disabled) return "disabled";
		if (stepIndex < currentStep) return "completed";
		if (stepIndex === currentStep) return "active";
		return "pending";
	};

	const renderStepIndicator = (step: Step, stepIndex: number) => {
		const state = getStepState(stepIndex);
		const Icon = step.icon;

		return (
			<div
				className={cn(stepIndicatorVariants({ size, state }))}
				aria-label={`${step.title} step indicator`}
			>
				{state === "completed" ? (
					<Check
						className={cn(
							size === "sm" ? "size-3" : size === "lg" ? "size-5" : "size-4",
						)}
					/>
				) : Icon ? (
					<Icon
						className={cn(
							size === "sm" ? "size-3" : size === "lg" ? "size-5" : "size-4",
						)}
					/>
				) : (
					<span
						className={cn(
							"font-medium",
							size === "sm"
								? "text-xs"
								: size === "lg"
									? "text-base"
									: "text-sm",
						)}
					>
						{stepIndex + 1}
					</span>
				)}
			</div>
		);
	};

	const renderStepContent = (step: Step, stepIndex: number) => {
		const state = getStepState(stepIndex);

		return (
			<div className="flex-1">
				<span
					className={cn(
						"font-medium tracking-tight",
						size === "sm" ? "text-xs" : size === "lg" ? "text-base" : "text-sm",
						state === "active"
							? "text-foreground"
							: state === "completed"
								? "text-success"
								: "text-muted-foreground",
					)}
				>
					{step.title}
					{step.optional && (
						<span className="ml-1 text-xs font-normal text-muted-foreground">
							(optional)
						</span>
					)}
				</span>
				{step.description && variant !== "compact" && (
					<div
						className={cn(
							"text-muted-foreground mt-0.5",
							size === "sm" ? "text-xs" : size === "lg" ? "text-sm" : "text-xs",
						)}
					>
						{step.description}
					</div>
				)}
				{state === "active" && (
					<div
						className={cn(
							"text-muted-foreground mt-0.5",
							size === "sm" ? "text-xs" : size === "lg" ? "text-sm" : "text-xs",
						)}
					>
						In progress
					</div>
				)}
			</div>
		);
	};

	const progressPercentage = ((currentStep + 1) / steps.length) * 100;

	return (
		<div className={cn(sidebarVariants({ size }), className)}>
			<div className="flex flex-col gap-6">
				{/* Header section */}
				{(title || TitleIcon) && (
					<>
						<div className="flex items-center gap-2.5">
							{TitleIcon && (
								<div className="p-2 rounded-xl bg-primary text-primary-foreground shadow-sm">
									<TitleIcon className="size-4" />
								</div>
							)}
							{title && (
								<span
									className={cn(
										"font-semibold tracking-tight text-foreground",
										size === "sm"
											? "text-base"
											: size === "lg"
												? "text-xl"
												: "text-lg",
									)}
								>
									{title}
								</span>
							)}
						</div>
						<Separator className="bg-border" />
					</>
				)}

				<div
					className={cn(
						"flex flex-col",
						variant === "compact" ? "gap-1" : "gap-1.5",
					)}
				>
					{steps.map((step, stepIndex) => {
						const state = getStepState(stepIndex);
						const isInteractive = canNavigateToStep(stepIndex);

						return (
							<div
								key={step.id}
								className={cn(
									stepItemVariants({
										size,
										state,
										interactive: isInteractive,
									}),
								)}
								onClick={() => isInteractive && goToStep(stepIndex)}
								role="tab"
								aria-selected={state === "active"}
								aria-current={state === "active" ? "step" : undefined}
								tabIndex={isInteractive ? 0 : undefined}
								onKeyDown={(e) => {
									if (isInteractive && (e.key === "Enter" || e.key === " ")) {
										e.preventDefault();
										goToStep(stepIndex);
									}
								}}
								data-slot="step"
							>
								{renderStepIndicator(step, stepIndex)}
								{renderStepContent(step, stepIndex)}
							</div>
						);
					})}
				</div>

				{/* Progress indicator */}
				{showProgress && (
					<div className="flex flex-col gap-2 pt-2">
						<div className="flex items-center justify-between">
							<span
								className={cn(
									"font-medium text-muted-foreground",
									size === "sm"
										? "text-xs"
										: size === "lg"
											? "text-sm"
											: "text-xs",
								)}
							>
								Progress
							</span>
							<span
								className={cn(
									"text-foreground font-medium",
									size === "sm"
										? "text-xs"
										: size === "lg"
											? "text-sm"
											: "text-xs",
								)}
							>
								{currentStep + 1} / {steps.length}
							</span>
						</div>
						<div
							className="w-full bg-muted rounded-full h-1.5"
							role="progressbar"
							aria-valuenow={progressPercentage}
							aria-valuemin={0}
							aria-valuemax={100}
							aria-label={`Progress: ${Math.round(progressPercentage)}%`}
							tabIndex={0}
						>
							<div
								className="bg-primary h-1.5 rounded-full transition-all duration-500 ease-out"
								style={{ width: `${progressPercentage}%` }}
								data-slot="progress-bar"
							/>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}

// VerticalStepperContent Component
interface VerticalStepperContentProps {
	className?: string;
	children: React.ReactNode;
}

function VerticalStepperContent({
	className,
	children,
}: VerticalStepperContentProps) {
	const { size } = useVerticalStepperContext();

	return (
		<ScrollArea
			className={cn(stepContentVariants({ size }), className)}
			data-slot="stepper-content"
		>
			{children}
		</ScrollArea>
	);
}

// VerticalStepperStep Component
interface VerticalStepperStepProps {
	id: string;
	className?: string;
	children?: React.ReactNode;
}

function VerticalStepperStep({
	id,
	className,
	children,
}: VerticalStepperStepProps) {
	const { currentStep, steps } = useVerticalStepperContext();
	const stepIndex = steps.findIndex((step) => step.id === id);
	const isActive = stepIndex === currentStep;

	if (!isActive) return null;

	return (
		<div
			className={className}
			role="tabpanel"
			aria-labelledby={`step-${id}`}
			data-slot="step-content"
		>
			{children}
		</div>
	);
}

const AnchorVerticalStepper = VerticalStepper;

VerticalStepper.displayName = "VerticalStepper";
VerticalStepperSidebar.displayName = "VerticalStepperSidebar";
VerticalStepperContent.displayName = "VerticalStepperContent";
VerticalStepperStep.displayName = "VerticalStepperStep";
AnchorVerticalStepper.displayName = "AnchorVerticalStepper";

export {
	VerticalStepper,
	VerticalStepperSidebar,
	VerticalStepperContent,
	VerticalStepperStep,
	AnchorVerticalStepper,
	useVerticalStepperContext,
	type Step,
	type VerticalStepperProps,
	type VerticalStepperStepProps,
	type VerticalStepperContextValue,
};
