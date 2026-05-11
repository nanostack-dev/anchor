import { SignupForm } from "@/components/auth/SignupForm";
import { Building2, Key, Shield, Users } from "lucide-react";

export function InitPage() {
	return (
		<div className="min-h-screen bg-gray-50 dark:bg-gray-950 flex flex-col">
			<div className="flex justify-center pt-8 pb-4">
				<img src="/logo.svg" alt="Anchor" className="h-12 w-auto" />
			</div>
			<div className="flex-1 flex items-center justify-center px-8">
				<div className="w-full max-w-4xl">
					<div className="text-center mb-12">
						<h1 className="text-4xl font-semibold text-gray-900 dark:text-white mb-4 tracking-tight">
							Welcome to Anchor
						</h1>

						<p className="text-lg text-gray-600 dark:text-gray-400 max-w-2xl mx-auto">
							Multi-tenant infrastructure for modern applications. Set up your
							organization and admin account to get started.
						</p>
					</div>

					<div className="grid lg:grid-cols-2 gap-12 items-start">
						{/* Left Side - Features */}
						<div className="space-y-8">
							<div className="space-y-6">
								<div className="flex items-start space-x-4">
									<div className="flex-shrink-0 w-10 h-10 bg-gray-100 dark:bg-gray-800 rounded-lg flex items-center justify-center">
										<Building2 className="w-5 h-5 text-gray-600 dark:text-gray-400" />
									</div>
									<div>
										<h3 className="font-medium text-gray-900 dark:text-white mb-1">
											Organizations & Workspaces
										</h3>
										<p className="text-sm text-gray-600 dark:text-gray-400">
											Structure your customers into hierarchical organizations
											and workspaces
										</p>
									</div>
								</div>

								<div className="flex items-start space-x-4">
									<div className="flex-shrink-0 w-10 h-10 bg-gray-100 dark:bg-gray-800 rounded-lg flex items-center justify-center">
										<Users className="w-5 h-5 text-gray-600 dark:text-gray-400" />
									</div>
									<div>
										<h3 className="font-medium text-gray-900 dark:text-white mb-1">
											User Management
										</h3>
										<p className="text-sm text-gray-600 dark:text-gray-400">
											Manage you products and their users with role-based access
											control
										</p>
									</div>
								</div>

								<div className="flex items-start space-x-4">
									<div className="flex-shrink-0 w-10 h-10 bg-gray-100 dark:bg-gray-800 rounded-lg flex items-center justify-center">
										<Shield className="w-5 h-5 text-gray-600 dark:text-gray-400" />
									</div>
									<div>
										<h3 className="font-medium text-gray-900 dark:text-white mb-1">
											Flexible Permissions
										</h3>
										<p className="text-sm text-gray-600 dark:text-gray-400">
											Role-based access control with granular permission
											management
										</p>
									</div>
								</div>
								<div className="flex items-start space-x-4">
									<div className="flex-shrink-0 w-10 h-10 bg-gray-100 dark:bg-gray-800 rounded-lg flex items-center justify-center">
										<Key className="w-5 h-5 text-gray-600 dark:text-gray-400" />
									</div>
									<div>
										<h3 className="font-medium text-gray-900 dark:text-white mb-1">
											Api Key Management
										</h3>
										<p className="text-sm text-gray-600 dark:text-gray-400">
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
