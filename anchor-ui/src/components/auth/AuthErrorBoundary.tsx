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
				return <WifiOff className="h-4 w-4" />;
			case "auth":
				return <AlertTriangle className="h-4 w-4" />;
			case "tenantservice":
				return <AlertTriangle className="h-4 w-4" />;
			case "token":
				return <AlertTriangle className="h-4 w-4" />;
			default:
				return <AlertTriangle className="h-4 w-4" />;
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
		<div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
			<Card className="w-full max-w-md">
				<CardHeader className="text-center">
					<div className="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-red-100 mb-4">
						{getErrorIcon()}
					</div>
					<CardTitle className="text-lg font-medium text-gray-900">
						{getErrorTitle()}
					</CardTitle>
					<CardDescription className="text-sm text-gray-600">
						{getErrorDescription()}
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4">
					<Alert variant="destructive">
						<AlertTriangle className="h-4 w-4" />
						<AlertTitle>Error Details</AlertTitle>
						<AlertDescription className="text-xs">
							{error.message}
							<br />
							<span className="text-gray-500">
								Occurred at: {new Date(error.timestamp).toLocaleString()}
							</span>
						</AlertDescription>
					</Alert>

					<div className="flex flex-col space-y-2">
						{shouldShowRetry && (
							<Button onClick={onRetry} className="w-full" variant="default">
								<RefreshCw className="mr-2 h-4 w-4" />
								Try Again
							</Button>
						)}

						<Button onClick={onLogout} variant="outline" className="w-full">
							<LogOut className="mr-2 h-4 w-4" />
							Logout
						</Button>

						{error.type === "auth" && (
							<Button
								onClick={goToLogin}
								variant="secondary"
								className="w-full"
							>
								<LogIn className="mr-2 h-4 w-4" />
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
