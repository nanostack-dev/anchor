import { SignupForm } from "@/components/auth/SignupForm";
import { Building2, Key, Shield, Users } from "lucide-react";

export function InitPage() {
	return (
		<div className="min-h-screen bg-background flex flex-col">
			<div className="flex justify-center pt-8 pb-4">
				<img src="/logo.svg" alt="Anchor" className="h-12 w-auto" />
			</div>
			<div className="flex-1 flex items-center justify-center px-8">
				<div className="w-full max-w-4xl">
					<div className="text-center mb-12">
						<h1 className="text-4xl font-semibold text-foreground mb-4 tracking-tight">
							Welcome to Anchor
						</h1>

						<p className="text-lg text-muted-foreground max-w-2xl mx-auto">
							Multi-tenant infrastructure for modern applications. Set up your
							organization and admin account to get started.
						</p>
					</div>

					<div className="grid lg:grid-cols-2 gap-12 items-start">
						{/* Left Side - Features */}
						<div className="flex flex-col gap-8">
							<div className="flex flex-col gap-6">
								<div className="flex items-start gap-4">
									<div className="flex-shrink-0 size-10 bg-muted rounded-lg flex items-center justify-center">
										<Building2 className="size-5 text-muted-foreground" />
									</div>
									<div>
										<h3 className="font-medium text-foreground mb-1">
											Organizations & Workspaces
										</h3>
										<p className="text-sm text-muted-foreground">
											Structure your customers into hierarchical organizations
											and workspaces
										</p>
									</div>
								</div>

								<div className="flex items-start gap-4">
									<div className="flex-shrink-0 size-10 bg-muted rounded-lg flex items-center justify-center">
										<Users className="size-5 text-muted-foreground" />
									</div>
									<div>
										<h3 className="font-medium text-foreground mb-1">
											User Management
										</h3>
										<p className="text-sm text-muted-foreground">
											Manage you products and their users with role-based access
											control
										</p>
									</div>
								</div>

								<div className="flex items-start gap-4">
									<div className="flex-shrink-0 size-10 bg-muted rounded-lg flex items-center justify-center">
										<Shield className="size-5 text-muted-foreground" />
									</div>
									<div>
										<h3 className="font-medium text-foreground mb-1">
											Flexible Permissions
										</h3>
										<p className="text-sm text-muted-foreground">
											Role-based access control with granular permission
											management
										</p>
									</div>
								</div>
								<div className="flex items-start gap-4">
									<div className="flex-shrink-0 size-10 bg-muted rounded-lg flex items-center justify-center">
										<Key className="size-5 text-muted-foreground" />
									</div>
									<div>
										<h3 className="font-medium text-foreground mb-1">
											Api Key Management
										</h3>
										<p className="text-sm text-muted-foreground">
											Securely manage your API keys with granular permissions
											and access control
										</p>
									</div>
								</div>
							</div>
						</div>
						<SignupForm variant="init" showLoginLink={false} />
					</div>
				</div>
			</div>
		</div>
	);
}
