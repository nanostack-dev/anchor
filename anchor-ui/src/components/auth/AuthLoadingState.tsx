import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Database, Loader2, Shield } from "lucide-react";
import type React from "react";

interface AuthLoadingStateProps {
	authLoading: boolean;
	tenantLoading: boolean;
	message?: string;
}

export const AuthLoadingState: React.FC<AuthLoadingStateProps> = ({
	authLoading,
	tenantLoading,
	message,
}) => {
	const getLoadingMessage = () => {
		if (message) return message;

		if (authLoading && tenantLoading) {
			return "Initializing application...";
		}
		if (authLoading) {
			return "Verifying authentication...";
		}
		if (tenantLoading) {
			return "Checking system status...";
		}

		return "Loading...";
	};

	const getLoadingIcon = () => {
		if (authLoading && tenantLoading) {
			return <Loader2 className="h-6 w-6 animate-spin" />;
		}
		if (authLoading) {
			return <Shield className="h-6 w-6 animate-pulse" />;
		}
		if (tenantLoading) {
			return <Database className="h-6 w-6 animate-pulse" />;
		}

		return <Loader2 className="h-6 w-6 animate-spin" />;
	};

	return (
		<div className="min-h-screen flex items-center justify-center bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
			<Card className="w-full max-w-md">
				<CardHeader className="text-center">
					<div className="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-blue-100 mb-4">
						{getLoadingIcon()}
					</div>
					<CardTitle className="text-lg font-medium text-gray-900">
						{getLoadingMessage()}
					</CardTitle>
					<CardDescription className="text-sm text-gray-600">
						Please wait while we set things up for you.
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4">
					<div className="space-y-3">
						<div className="flex items-center space-x-3">
							<div
								className={`w-2 h-2 rounded-full ${
									authLoading ? "bg-blue-500 animate-pulse" : "bg-green-500"
								}`}
							/>
							<span className="text-sm text-gray-600">
								Authentication {authLoading ? "in progress..." : "verified"}
							</span>
						</div>

						<div className="flex items-center space-x-3">
							<div
								className={`w-2 h-2 rounded-full ${
									tenantLoading ? "bg-blue-500 animate-pulse" : "bg-green-500"
								}`}
							/>
							<span className="text-sm text-gray-600">
								System status {tenantLoading ? "checking..." : "ready"}
							</span>
						</div>
					</div>

					<div className="space-y-2">
						<Skeleton className="h-4 w-full" />
						<Skeleton className="h-4 w-3/4" />
						<Skeleton className="h-4 w-1/2" />
					</div>
				</CardContent>
			</Card>
		</div>
	);
};
