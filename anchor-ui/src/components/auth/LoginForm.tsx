import { loginMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Button } from "@nanostackorg/design-system/components/button";
import { Card, CardContent } from "@nanostackorg/design-system/components/card";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";

import { type AuthClaims, useAuth } from "@/context/auth/AuthContext";
import { loginRoute } from "@/routes/platform/login";
import { registerRoute } from "@/routes/platform/register";
import { TextLink } from "@nanostackorg/design-system/components/text-link";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useForm } from "@tanstack/react-form";
import { useMutation } from "@tanstack/react-query";
import { useSearch } from "@tanstack/react-router";
import { jwtDecode } from "jwt-decode";
import { useId } from "react";
import { flushSync } from "react-dom";
import { z } from "zod";

const loginFormSchema = z.object({
	email: z.email("Invalid email address"),
	password: z.string().min(1, "Password is required"),
});

type LoginFormData = z.infer<typeof loginFormSchema>;

export function LoginForm() {
	const formId = useId();
	const { login, handleSuccessfulAuth } = useAuth();
	const searchParams = useSearch({ from: loginRoute.id });

	const form = useForm({
		defaultValues: {
			email: "",
			password: "",
		} as LoginFormData,
		onSubmit: async ({ value }) => {
			const result = loginFormSchema.safeParse(value);
			if (!result.success) {
				return;
			}
			await onSubmit(value);
		},
		validators: {
			onChange: loginFormSchema,
			onSubmit: loginFormSchema,
		},
	});

	const { mutate: loginUser, isPending: isLoggingIn } = useMutation({
		...loginMutation({
			credentials: "include",
		}),
		onSuccess: (data) => {
			toast.add({ type: "success", title: "Login successful!" });
			try {
				const claims = jwtDecode(data.accessToken);
				flushSync(() => login(data.accessToken, claims as AuthClaims));
				handleSuccessfulAuth(searchParams.redirect);
			} catch (e) {
				toast.add({ type: "error", title: "Failed to decode login token." });
				return;
			}
		},
		onError: (err) => {
			if (err.errors && err.errors.length > 0) {
				const errorMessage = err.errors[0].message;
				toast.add({ type: "error", title: errorMessage });
				return;
			}

			toast.add({ type: "error", title: "An error occurred during login." });
		},
	});

	const onSubmit = async (values: LoginFormData) => {
		loginUser({
			body: {
				email: values.email,
				password: values.password,
			},
		});
	};

	return (
		<Card variant="outline">
			<CardContent>
				<Stack space="lg">
					<div className="mb-6">
						<Heading level={1}>Login to your account</Heading>
						<p className="text-sm text-muted-foreground">
							Enter your email below to login to your account
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
						<form.Field name="email">
							{(field) => (
								<div className="space-y-2">
									<Label htmlFor={`${formId}-email`}>Email</Label>
									<Input
										id={`${formId}-email`}
										type="email"
										placeholder="m@example.com"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										disabled={isLoggingIn}
									/>
									<FormValidationError field={field} />
								</div>
							)}
						</form.Field>

						<form.Field name="password">
							{(field) => (
								<div className="space-y-2">
									<div className="flex items-center justify-between">
										<Label htmlFor={`${formId}-password`}>Password</Label>
										<TextLink href="/login">Forgot your password?</TextLink>
									</div>
									<Input
										id={`${formId}-password`}
										type="password"
										placeholder="••••••••"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										disabled={isLoggingIn}
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
									loading={isLoggingIn || isSubmitting}
								>
									{isLoggingIn || isSubmitting ? "Logging in..." : "Login"}
								</Button>
							)}
						</form.Subscribe>
					</form>

					<div className="text-center text-sm mt-6">
						Don't have an account?{" "}
						<TextLink
							href={`${registerRoute.fullPath}${searchParams.redirect ? `?${new URLSearchParams({ redirect: searchParams.redirect })}` : ""}`}
						>
							Sign up
						</TextLink>
					</div>
				</Stack>
			</CardContent>
		</Card>
	);
}
