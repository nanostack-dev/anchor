import type { AuthError } from "@/context/auth/AuthContext";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
} from "@nanostackorg/design-system/components/card";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { AlertTriangle, LogIn, LogOut, RefreshCw, WifiOff } from "lucide-react";
import type React from "react";

interface AuthErrorBoundaryProps {
	error: AuthError;
	onRetry: () => void;
	onLogout: () => void;
	children?: React.ReactNode;
}

export const AuthErrorBoundary: React.FC<AuthErrorBoundaryProps> = ({
	error,
	onRetry,
	onLogout,
	children,
}) => {
	const getErrorIcon = () => {
		switch (error.type) {
			case "network":
				return <WifiOff className="size-4" />;
			case "auth":
				return <AlertTriangle className="size-4" />;
			case "tenantservice":
				return <AlertTriangle className="size-4" />;
			case "token":
				return <AlertTriangle className="size-4" />;
			default:
				return <AlertTriangle className="size-4" />;
		}
	};

	const getErrorTitle = () => {
		switch (error.type) {
			case "network":
				return "Connection Error";
			case "auth":
				return "Authentication Failed";
			case "tenantservice":
				return "Service Unavailable";
			case "token":
				return "Session Expired";
			default:
				return "Something went wrong";
		}
	};

	const getErrorDescription = () => {
		switch (error.type) {
			case "network":
				return "Unable to connect to the server. Please check your internet connection and try again.";
			case "auth":
				return "Your session has expired or authentication failed. Please log in again.";
			case "tenantservice":
				return "The service is temporarily unavailable. Please try again in a few moments.";
			case "token":
				return "Your session has expired. Please refresh the page or log in again.";
			default:
				return (
					error.message || "An unexpected error occurred. Please try again."
				);
		}
	};

	const shouldShowRetry = error.retryable;
	const goToLogin = () => {
		window.location.href = "/login";
	};

	return (
		<div className="flex min-h-screen items-center justify-center bg-muted px-4 py-12 sm:px-6 lg:px-8">
			<Box className="w-full max-w-md">
				<Card variant="outline">
					<CardHeader>
						<div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-destructive/10 text-destructive">
							{getErrorIcon()}
						</div>
						<Heading level={1}>{getErrorTitle()}</Heading>
						<CardDescription>{getErrorDescription()}</CardDescription>
					</CardHeader>
					<CardContent>
						<Stack space="md">
							<Alert tone="critical">
								<AlertTriangle className="size-4" />
								<AlertTitle>Error Details</AlertTitle>
								<AlertDescription>
									{error.message}
									<br />
									<span className="text-muted-foreground break-words">
										Occurred at: {new Date(error.timestamp).toLocaleString()}
									</span>
								</AlertDescription>
							</Alert>

							<div className="flex flex-col gap-2">
								{shouldShowRetry && (
									<Button
										onClick={onRetry}
										width="fill"
										variant="solid"
										tone="brand"
									>
										<RefreshCw data-icon="inline-start" />
										Try Again
									</Button>
								)}

								<Button onClick={onLogout} variant="outline" width="fill">
									<LogOut data-icon="inline-start" />
									Logout
								</Button>

								{error.type === "auth" && (
									<Button onClick={goToLogin} variant="soft" width="fill">
										<LogIn data-icon="inline-start" />
										Go to Login
									</Button>
								)}
							</div>
						</Stack>
					</CardContent>
				</Card>
			</Box>

			{children}
		</div>
	);
};
