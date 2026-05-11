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
			compact: "space-y-2",
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

const sidebarVariants = cva("bg-slate-50/80 border-r border-slate-200/70 p-6", {
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
				sm: "h-7 w-7",
				default: "h-8 w-8",
				lg: "h-9 w-9",
			},
			state: {
				pending: "bg-slate-100 text-slate-400 border border-slate-200",
				active: "bg-slate-900 text-white shadow-sm",
				completed: "bg-emerald-500 text-white",
				disabled:
					"bg-slate-100 text-slate-300 border border-slate-200 opacity-50",
			},
		},
		defaultVariants: {
			size: "default",
			state: "pending",
		},
	},
);

const stepItemVariants = cva(
	"flex items-center space-x-3 px-3 py-2.5 rounded-xl transition-all duration-200",
	{
		variants: {
			size: {
				sm: "px-2 py-2 space-x-2",
				default: "px-3 py-2.5 space-x-3",
				lg: "px-4 py-3 space-x-4",
			},
			state: {
				pending: "text-slate-400",
				active: "bg-white shadow-sm border border-slate-200/80 text-slate-900",
				completed: "text-emerald-600",
				disabled: "text-slate-300 opacity-50",
			},
			interactive: {
				true: "cursor-pointer hover:bg-white/70",
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
							size === "sm" ? "h-3 w-3" : size === "lg" ? "h-5 w-5" : "h-4 w-4",
						)}
					/>
				) : Icon ? (
					<Icon
						className={cn(
							size === "sm" ? "h-3 w-3" : size === "lg" ? "h-5 w-5" : "h-4 w-4",
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
							? "text-slate-900"
							: state === "completed"
								? "text-emerald-600"
								: "text-slate-400",
					)}
				>
					{step.title}
					{step.optional && (
						<span className="ml-1 text-xs font-normal text-slate-400">
							(optional)
						</span>
					)}
				</span>
				{step.description && variant !== "compact" && (
					<div
						className={cn(
							"text-slate-400 mt-0.5",
							size === "sm" ? "text-xs" : size === "lg" ? "text-sm" : "text-xs",
						)}
					>
						{step.description}
					</div>
				)}
				{state === "active" && (
					<div
						className={cn(
							"text-slate-400 mt-0.5",
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
			<div className="space-y-6">
				{/* Header section */}
				{(title || TitleIcon) && (
					<>
						<div className="flex items-center space-x-2.5">
							{TitleIcon && (
								<div className="p-2 rounded-xl bg-slate-900 text-white shadow-sm">
									<TitleIcon className="h-4 w-4" />
								</div>
							)}
							{title && (
								<span
									className={cn(
										"font-semibold tracking-tight text-slate-900",
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
						<Separator className="bg-slate-200/70" />
					</>
				)}

				<div
					className={cn(variant === "compact" ? "space-y-1" : "space-y-1.5")}
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
					<div className="space-y-2 pt-2">
						<div className="flex items-center justify-between">
							<span
								className={cn(
									"font-medium text-slate-400",
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
									"text-slate-500 font-medium",
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
							className="w-full bg-slate-200 rounded-full h-1.5"
							role="progressbar"
							aria-valuenow={progressPercentage}
							aria-valuemin={0}
							aria-valuemax={100}
							aria-label={`Progress: ${Math.round(progressPercentage)}%`}
							tabIndex={0}
						>
							<div
								className="bg-slate-900 h-1.5 rounded-full transition-all duration-500 ease-out"
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
