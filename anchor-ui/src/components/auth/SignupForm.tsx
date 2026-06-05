import { registerMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { type AuthClaims, useAuth } from "@/context/auth/AuthContext";
import { loginRoute } from "@/routes/platform/login";
import { useForm } from "@tanstack/react-form";
import { useMutation } from "@tanstack/react-query";
import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import { jwtDecode } from "jwt-decode";
import type * as React from "react";
import { toast } from "sonner";
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

interface SignupFormProps extends React.ComponentPropsWithoutRef<"div"> {
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
	className,
	variant = "register",
	title,
	description,
	submitText,
	showLoginLink = true,
	...props
}: SignupFormProps) {
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
			toast.success(successMessage);

			if (!data.accessToken) {
				toast.error(
					"Registration succeeded, but no session was returned. Please sign in.",
				);
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
				toast.error(errorMessage);
				navigate({ to: "/login" });
			}
		},
		onError: (err) => {
			if (err.errors && err.errors.length > 0) {
				const errorMessage = err.errors[0].message;
				toast.error(errorMessage);
				return;
			}

			const errorMessage = isInit
				? "An error occurred during setup."
				: "An error occurred during registration.";
			toast.error(errorMessage);
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
		<Card className={className} {...props}>
			<CardContent className="p-8">
				<div className="mb-6">
					<h2 className="text-xl font-semibold text-foreground mb-2">
						{title || defaultTitle}
					</h2>
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
									<Label>Organization Name</Label>
									<Input
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
								<Label>{isInit ? "Administrator Email" : "Email"}</Label>
								<Input
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
								<Label>Password</Label>
								<Input
									type="password"
									placeholder="••••••••"
									value={field.state.value}
									onChange={(e) => field.handleChange(e.target.value)}
									onBlur={field.handleBlur}
									disabled={isRegistering}
								/>
								<FormValidationError field={field} />
								<p className="text-xs text-muted-foreground">
									Must contain uppercase, lowercase, and number. At least 8
									characters.
								</p>
							</div>
						)}
					</form.Field>
					<form.Field name="confirmPassword">
						{(field) => (
							<div className="space-y-2">
								<Label>Confirm Password</Label>
								<Input
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
								type="submit"
								disabled={
									!canSubmit ||
									isSubmitting ||
									!isValid ||
									isValidating ||
									!isDirty
								}
								className="w-full h-11"
							>
								{isRegistering || isSubmitting ? (
									<div className="flex items-center gap-2">
										<Spinner className="text-current" />
										<span>
											{isInit ? "Setting up Anchor..." : "Creating Account..."}
										</span>
									</div>
								) : (
									<div className="flex items-center gap-2">
										<span>{submitText || defaultSubmitText}</span>
									</div>
								)}
							</Button>
						)}
					</form.Subscribe>
				</form>

				{showLoginLink && (
					<div className="text-center text-sm mt-6">
						Already have an account?{" "}
						<Link
							to={loginRoute.fullPath}
							search={redirect ? { redirect } : {}}
							className="underline underline-offset-4 text-primary hover:opacity-80"
						>
							Sign in
						</Link>
					</div>
				)}
			</CardContent>
		</Card>
	);
}
