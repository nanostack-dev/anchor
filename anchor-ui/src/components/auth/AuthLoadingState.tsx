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
			return <Loader2 className="size-6 animate-spin" />;
		}
		if (authLoading) {
			return <Shield className="size-6 animate-pulse" />;
		}
		if (tenantLoading) {
			return <Database className="size-6 animate-pulse" />;
		}

		return <Loader2 className="size-6 animate-spin" />;
	};

	return (
		<div className="flex min-h-screen items-center justify-center bg-muted px-4 py-12 sm:px-6 lg:px-8">
			<Card className="w-72 min-w-0 max-w-[calc(100vw-2rem)] sm:w-full sm:max-w-md">
				<CardHeader className="text-center">
					<div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-accent-soft text-accent-foreground">
						{getLoadingIcon()}
					</div>
					<CardTitle className="text-lg font-medium text-foreground">
						{getLoadingMessage()}
					</CardTitle>
					<CardDescription className="text-sm text-muted-foreground">
						Please wait while we set things up for you.
					</CardDescription>
				</CardHeader>
				<CardContent className="flex flex-col gap-4">
					<div className="flex flex-col gap-3">
						<div className="flex items-center gap-3">
							<div
								className={`size-2 rounded-full ${
									authLoading ? "bg-primary animate-pulse" : "bg-success"
								}`}
							/>
							<span className="text-sm text-muted-foreground">
								Authentication {authLoading ? "in progress..." : "verified"}
							</span>
						</div>

						<div className="flex items-center gap-3">
							<div
								className={`size-2 rounded-full ${
									tenantLoading ? "bg-primary animate-pulse" : "bg-success"
								}`}
							/>
							<span className="text-sm text-muted-foreground">
								System status {tenantLoading ? "checking..." : "ready"}
							</span>
						</div>
					</div>

					<div className="flex flex-col gap-2">
						<Skeleton className="h-4 w-full" />
						<Skeleton className="h-4 w-3/4" />
						<Skeleton className="h-4 w-1/2" />
					</div>
				</CardContent>
			</Card>
		</div>
	);
};
