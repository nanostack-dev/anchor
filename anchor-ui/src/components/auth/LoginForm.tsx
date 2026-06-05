import { loginMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { type AuthClaims, useAuth } from "@/context/auth/AuthContext";
import { loginRoute } from "@/routes/platform/login";
import { registerRoute } from "@/routes/platform/register";
import { useForm } from "@tanstack/react-form";
import { useMutation } from "@tanstack/react-query";
import { Link, useSearch } from "@tanstack/react-router";
import { jwtDecode } from "jwt-decode";
import type * as React from "react";
import { toast } from "sonner";
import { z } from "zod";

const loginFormSchema = z.object({
	email: z.email("Invalid email address"),
	password: z.string().min(1, "Password is required"),
});

type LoginFormData = z.infer<typeof loginFormSchema>;

interface LoginFormProps extends React.ComponentPropsWithoutRef<"div"> {}

export function LoginForm({ className, ...props }: LoginFormProps) {
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
			toast.success("Login successful!");
			try {
				const claims = jwtDecode(data.accessToken);
				login(data.accessToken, claims as AuthClaims);
				handleSuccessfulAuth(searchParams.redirect);
			} catch (e) {
				toast.error("Failed to decode login token.");
				return;
			}
		},
		onError: (err) => {
			if (err.errors && err.errors.length > 0) {
				const errorMessage = err.errors[0].message;
				toast.error(errorMessage);
				return;
			}

			toast.error("An error occurred during login.");
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
		<Card className={className} {...props}>
			<CardContent className="p-8">
				<div className="mb-6">
					<h2 className="text-xl font-semibold text-foreground mb-2">
						Login to your account
					</h2>
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
								<Label>Email</Label>
								<Input
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
									<Label>Password</Label>
									<Link
										to="/login"
										className="text-sm text-primary underline-offset-4 hover:underline"
									>
										Forgot your password?
									</Link>
								</div>
								<Input
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
								{isLoggingIn || isSubmitting ? (
									<div className="flex items-center gap-2">
										<Spinner className="text-current" />
										<span>Logging in...</span>
									</div>
								) : (
									<span>Login</span>
								)}
							</Button>
						)}
					</form.Subscribe>
				</form>

				<div className="text-center text-sm mt-6">
					Don't have an account?{" "}
					<Link
						to={registerRoute.fullPath}
						search={
							searchParams.redirect ? { redirect: searchParams.redirect } : {}
						}
						className="underline underline-offset-4 text-primary hover:opacity-80"
					>
						Sign up
					</Link>
				</div>
			</CardContent>
		</Card>
	);
}
