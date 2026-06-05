import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import type { AuthError } from "@/context/auth/AuthContext";
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
			<Card className="w-72 min-w-0 max-w-[calc(100vw-2rem)] sm:w-full sm:max-w-md">
				<CardHeader className="text-center">
					<div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-destructive/10 text-destructive">
						{getErrorIcon()}
					</div>
					<CardTitle className="text-lg font-medium text-foreground">
						{getErrorTitle()}
					</CardTitle>
					<CardDescription className="text-sm text-muted-foreground">
						{getErrorDescription()}
					</CardDescription>
				</CardHeader>
				<CardContent className="flex flex-col gap-4">
					<Alert variant="destructive" className="min-w-0">
						<AlertTriangle className="size-4" />
						<AlertTitle>Error Details</AlertTitle>
						<AlertDescription className="min-w-0 break-words text-xs">
							{error.message}
							<br />
							<span className="text-muted-foreground break-words">
								Occurred at: {new Date(error.timestamp).toLocaleString()}
							</span>
						</AlertDescription>
					</Alert>

					<div className="flex flex-col gap-2">
						{shouldShowRetry && (
							<Button onClick={onRetry} className="w-full" variant="default">
								<RefreshCw data-icon="inline-start" />
								Try Again
							</Button>
						)}

						<Button onClick={onLogout} variant="outline" className="w-full">
							<LogOut data-icon="inline-start" />
							Logout
						</Button>

						{error.type === "auth" && (
							<Button
								onClick={goToLogin}
								variant="secondary"
								className="w-full"
							>
								<LogIn data-icon="inline-start" />
								Go to Login
							</Button>
						)}
					</div>
				</CardContent>
			</Card>

			{children}
		</div>
	);
};
