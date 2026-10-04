import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { Database, Loader2, Shield } from "lucide-react";
import type React from "react";
import {
	AnchorAnimatedBackdrop,
	FloatingAnchorMark,
} from "./LoginAnimationPanel";

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
		<div className="relative min-h-screen overflow-hidden">
			<AnchorAnimatedBackdrop>
				<div className="absolute inset-0 flex flex-col items-center justify-center gap-10 px-4 py-12 sm:px-6 lg:px-8">
					<FloatingAnchorMark size="md" />
					<div className="w-full max-w-sm">
						<Card variant="outline">
							<CardHeader>
								<div className="mx-auto mb-4 flex size-12 items-center justify-center rounded-full bg-accent text-accent-foreground">
									{getLoadingIcon()}
								</div>
								<CardTitle>{getLoadingMessage()}</CardTitle>
								<CardDescription>
									Please wait while we set things up for you.
								</CardDescription>
							</CardHeader>
							<CardContent>
								<Stack space="md">
									<div className="flex flex-col gap-3">
										<div className="flex items-center gap-3">
											<div
												className={`size-2 rounded-full ${
													authLoading
														? "bg-primary animate-pulse"
														: "bg-success"
												}`}
											/>
											<span className="text-sm text-muted-foreground">
												Authentication{" "}
												{authLoading ? "in progress..." : "verified"}
											</span>
										</div>

										<div className="flex items-center gap-3">
											<div
												className={`size-2 rounded-full ${
													tenantLoading
														? "bg-primary animate-pulse"
														: "bg-success"
												}`}
											/>
											<span className="text-sm text-muted-foreground">
												System status {tenantLoading ? "checking..." : "ready"}
											</span>
										</div>
									</div>

									<div className="flex flex-col gap-2">
										<Skeleton height="sm" />
										<Skeleton height="sm" width="3/4" />
										<Skeleton height="sm" width="1/2" />
									</div>
								</Stack>
							</CardContent>
						</Card>
					</div>
				</div>
			</AnchorAnimatedBackdrop>
		</div>
	);
};
