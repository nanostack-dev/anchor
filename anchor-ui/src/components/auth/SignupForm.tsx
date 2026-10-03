import { registerMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Button } from "@nanostackorg/design-system/components/button";
import { Card, CardContent } from "@nanostackorg/design-system/components/card";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";

import { type AuthClaims, useAuth } from "@/context/auth/AuthContext";
import { loginRoute } from "@/routes/platform/login";
import { TextLink } from "@nanostackorg/design-system/components/text-link";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useForm } from "@tanstack/react-form";
import { useMutation } from "@tanstack/react-query";
import { useNavigate, useRouterState } from "@tanstack/react-router";
import { jwtDecode } from "jwt-decode";
import { useId } from "react";
import { z } from "zod";

const signupFormSchema = z
	.object({
		email: z.email("Invalid email address"),
		password: z
			.string()
			.min(8, "Password must be at least 8 characters")
			.regex(
				/^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[^A-Za-z\d]).+$/,
				"Password must contain at least one uppercase letter, one lowercase letter, one digit, and one special character",
			),
		confirmPassword: z.string().min(1, "Please confirm your password"),
		organizationName: z
			.string()
			.min(2, "Organization name must be at least 2 characters")
			.max(100, "Organization name must be less than 100 characters")
			.trim()
			.optional(),
	})
	.refine((data) => data.password === data.confirmPassword, {
		message: "Passwords do not match",
		path: ["confirmPassword"],
	});

type SignupFormData = z.infer<typeof signupFormSchema>;

interface SignupFormProps {
	variant?: "register" | "init";
	email?: string;
	tenantId?: string;
	invitationCode?: string;
	title?: string;
	description?: string;
	submitText?: string;
	showLoginLink?: boolean;
}

export function SignupForm({
	variant = "register",
	title,
	description,
	submitText,
	showLoginLink = true,
	...props
}: SignupFormProps) {
	const formId = useId();
	const navigate = useNavigate();
	const { login, handleSuccessfulAuth } = useAuth();
	const redirect = useRouterState({
		select: (state) => {
			const search = state.location.search as { redirect?: unknown };
			return typeof search.redirect === "string" ? search.redirect : undefined;
		},
	});

	const isInit = variant === "init";

	const form = useForm({
		defaultValues: {
			email: props.email || "",
			password: "",
			confirmPassword: "",
			...(isInit && { organizationName: "" }),
		} as SignupFormData,
		onSubmit: async ({ value }) => {
			const result = signupFormSchema.safeParse(value);
			if (!result.success) {
				return;
			}
			await onSubmit(value);
		},
		validators: {
			onChange: signupFormSchema,
			onSubmit: signupFormSchema,
		},
	});

	const { mutate: registerUser, isPending: isRegistering } = useMutation({
		...registerMutation({ credentials: "include" }),
		onSuccess: (data) => {
			const successMessage = isInit
				? "Anchor is ready! Welcome aboard."
				: "Registration successful!";
			toast.add({ type: "success", title: successMessage });

			if (!data.accessToken) {
				toast.add({
					type: "error",
					title:
						"Registration succeeded, but no session was returned. Please sign in.",
				});
				navigate({ to: "/login" });
				return;
			}

			try {
				const claims = jwtDecode(data.accessToken);
				login(data.accessToken, claims as AuthClaims);

				if (isInit) {
					navigate({ to: "/" });
				} else {
					handleSuccessfulAuth(redirect);
				}
			} catch (e) {
				const errorMessage = isInit
					? "Setup complete but failed to login automatically."
					: "Failed to decode registration token.";
				toast.add({ type: "error", title: errorMessage });
				navigate({ to: "/login" });
			}
		},
		onError: (err) => {
			if (err.errors && err.errors.length > 0) {
				const errorMessage = err.errors[0].message;
				toast.add({ type: "error", title: errorMessage });
				return;
			}

			const errorMessage = isInit
				? "An error occurred during setup."
				: "An error occurred during registration.";
			toast.add({ type: "error", title: errorMessage });
		},
	});

	const onSubmit = async (values: SignupFormData) => {
		const baseData = {
			email: values.email,
			password: values.password,
			invitation_code: props.invitationCode,
		};

		const requestData =
			isInit && "organizationName" in values
				? { ...baseData, tenant_name: values.organizationName }
				: baseData;

		registerUser({
			body: requestData,
		});
	};

	const defaultTitle = isInit ? "Setup Your Platform" : "Create an account";
	const defaultDescription = isInit
		? "Create your organization and administrator account"
		: "Enter your details below to create your account";
	const defaultSubmitText = isInit ? "Launch Anchor" : "Create account";

	return (
		<Card variant="outline">
			<CardContent>
				<Stack space="lg">
					<div className="mb-6">
						<Heading level={1}>{title || defaultTitle}</Heading>
						<p className="text-sm text-muted-foreground">
							{description || defaultDescription}
						</p>
					</div>

					<form
						onSubmit={(e) => {
							e.preventDefault();
							e.stopPropagation();
							form.handleSubmit();
						}}
						className="space-y-6"
					>
						{isInit && (
							<form.Field name="organizationName">
								{(field) => (
									<div className="space-y-2">
										<Label htmlFor={`${formId}-organizationName`}>
											Organization Name
										</Label>
										<Input
											id={`${formId}-organizationName`}
											placeholder="Acme Corporation"
											value={field.state.value}
											onChange={(e) => field.handleChange(e.target.value)}
											onBlur={field.handleBlur}
											disabled={isRegistering}
										/>
										<FormValidationError field={field} />
									</div>
								)}
							</form.Field>
						)}

						<form.Field name="email">
							{(field) => (
								<div className="space-y-2">
									<Label htmlFor={`${formId}-email`}>
										{isInit ? "Administrator Email" : "Email"}
									</Label>
									<Input
										id={`${formId}-email`}
										type="email"
										placeholder={isInit ? "admin@example.com" : "m@example.com"}
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										disabled={isRegistering || !!props.email}
									/>
									<FormValidationError field={field} />
								</div>
							)}
						</form.Field>

						<form.Field name="password">
							{(field) => (
								<div className="space-y-2">
									<Label htmlFor={`${formId}-password`}>Password</Label>
									<Input
										id={`${formId}-password`}
										type="password"
										placeholder="••••••••"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										disabled={isRegistering}
									/>
									<FormValidationError field={field} />
									<p className="text-xs text-muted-foreground">
										Must contain uppercase, lowercase, a number, and a special
										character. At least 8 characters.
									</p>
								</div>
							)}
						</form.Field>
						<form.Field name="confirmPassword">
							{(field) => (
								<div className="space-y-2">
									<Label htmlFor={`${formId}-confirmPassword`}>
										Confirm Password
									</Label>
									<Input
										id={`${formId}-confirmPassword`}
										type="password"
										placeholder="••••••••"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										disabled={isRegistering}
									/>
									<FormValidationError field={field} />
								</div>
							)}
						</form.Field>

						<form.Subscribe
							selector={(state) => [
								state.canSubmit,
								state.isSubmitting,
								state.isDirty,
								state.isValidating,
								state.isValid,
							]}
						>
							{([canSubmit, isSubmitting, isDirty, isValidating, isValid]) => (
								<Button
									variant="solid"
									tone="brand"
									type="submit"
									disabled={
										!canSubmit ||
										isSubmitting ||
										!isValid ||
										isValidating ||
										!isDirty
									}
									width="fill"
									size="lg"
									loading={isRegistering || isSubmitting}
								>
									{isRegistering || isSubmitting
										? isInit
											? "Setting up Anchor..."
											: "Creating Account..."
										: submitText || defaultSubmitText}
								</Button>
							)}
						</form.Subscribe>
					</form>

					{showLoginLink && (
						<div className="text-center text-sm mt-6">
							Already have an account?{" "}
							<TextLink
								href={`${loginRoute.fullPath}${redirect ? `?${new URLSearchParams({ redirect })}` : ""}`}
							>
								Sign in
							</TextLink>
						</div>
					)}
				</Stack>
			</CardContent>
		</Card>
	);
}
