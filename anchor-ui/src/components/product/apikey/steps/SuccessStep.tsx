import type { CreatedProductApiKeyResponse } from "@/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { ScrollArea } from "@/components/ui/scroll-area";
import { VerticalStepperStep } from "@/components/ui/vertical-stepper";
import {
	AlertCircle,
	Check,
	CheckCircle,
	Copy,
	Eye,
	EyeOff,
	Lock,
} from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

interface SuccessStepProps {
	createdApiKey: CreatedProductApiKeyResponse;
	onClose: () => void;
}

export function SuccessStep({ createdApiKey, onClose }: SuccessStepProps) {
	const [showApiKeyValue, setShowApiKeyValue] = useState(false);
	const [apiKeyCopied, setApiKeyCopied] = useState(false);

	const copyToClipboard = async () => {
		if (createdApiKey?.value) {
			try {
				await navigator.clipboard.writeText(createdApiKey.value);
				setApiKeyCopied(true);
				toast.success("API key copied to clipboard!");
				setTimeout(() => setApiKeyCopied(false), 2000);
			} catch (err) {
				toast.error("Failed to copy API key");
			}
		}
	};

	return (
		<VerticalStepperStep id="success">
			<div className="flex flex-col h-full">
				{/* Fixed Header */}
				<div className="px-6 pt-6 pb-4">
					<DialogHeader className="space-y-3">
						<DialogTitle className="flex items-center space-x-3">
							<div className="p-2 rounded-lg bg-primary text-primary-foreground">
								<CheckCircle className="h-5 w-5" />
							</div>
							<span className="text-xl">Success</span>
						</DialogTitle>
						<DialogDescription className="text-base">
							Your API key has been created successfully
						</DialogDescription>
					</DialogHeader>
				</div>

				{/* Scrollable Content */}
				<ScrollArea className="flex-1 px-6">
					<div className="space-y-6 pb-6">
						<div className="p-6 rounded-xl border bg-green-50/50 dark:bg-green-900/10 border-green-200 dark:border-green-800">
							<div className="flex items-center space-x-3 mb-3">
								<div className="p-2 rounded-lg bg-green-100 dark:bg-green-900/30">
									<CheckCircle className="h-5 w-5 text-green-600" />
								</div>
								<h4 className="text-lg font-semibold text-green-800 dark:text-green-200">
									API Key Created Successfully!
								</h4>
							</div>
							<p className="text-sm text-green-600 dark:text-green-300">
								Your API key has been created and configured with the selected
								permissions.
							</p>
						</div>

						<div className="p-6 rounded-xl border bg-yellow-50/50 dark:bg-yellow-900/10 border-yellow-200 dark:border-yellow-800">
							<div className="flex items-start space-x-3 mb-4">
								<div className="p-2 rounded-lg bg-yellow-100 dark:bg-yellow-900/30">
									<AlertCircle className="h-5 w-5 text-yellow-600" />
								</div>
								<div>
									<p className="text-sm font-semibold text-yellow-800 dark:text-yellow-200">
										Important: Copy Your API Key
									</p>
									<p className="text-sm mt-1 text-yellow-600 dark:text-yellow-300">
										This is the only time you'll see the full API key value.
										Make sure to copy and store it securely.
									</p>
								</div>
							</div>

							<div className="space-y-4">
								<div>
									<Label className="text-sm font-semibold text-muted-foreground">
										API Key Name
									</Label>
									<p className="text-lg font-medium mt-1">
										{createdApiKey.name || "No name"}
									</p>
								</div>

								<div>
									<Label className="text-sm font-semibold text-muted-foreground">
										Permissions mutability
									</Label>
									<p className="text-sm mt-1">
										{createdApiKey.mutable ? "Mutable" : "Immutable"}
									</p>
								</div>

								<div>
									<Label className="text-sm font-semibold text-muted-foreground">
										API Key Value
									</Label>
									<div className="flex items-center space-x-2 mt-2">
										<div className="flex-1 p-3 bg-muted rounded-lg border font-mono text-sm  break-all">
											{showApiKeyValue
												? createdApiKey.value || "No value"
												: "••••••••••••••••••••••••••••••••••••••••"}
										</div>
										<Button
											type="button"
											variant="outline"
											size="icon"
											onClick={() => setShowApiKeyValue(!showApiKeyValue)}
											className="shrink-0"
										>
											{showApiKeyValue ? (
												<EyeOff className="h-4 w-4" />
											) : (
												<Eye className="h-4 w-4" />
											)}
										</Button>
										<Button
											type="button"
											variant="outline"
											size="icon"
											onClick={copyToClipboard}
											className="shrink-0"
										>
											<Copy className="h-4 w-4" />
										</Button>
									</div>
									{apiKeyCopied && (
										<p className="text-sm text-green-600 mt-1 flex items-center">
											<Check className="h-3 w-3 mr-1" />
											Copied to clipboard!
										</p>
									)}
								</div>

								{createdApiKey.permissions &&
									createdApiKey.permissions.length > 0 && (
										<div>
											<Label className="text-sm font-semibold text-muted-foreground">
												Assigned Permissions ({createdApiKey.permissions.length}
												)
											</Label>
											<div className="flex flex-wrap gap-2 mt-2">
												{createdApiKey.permissions.map((permission) => (
													<Badge
														key={permission.permission_name}
														variant="outline"
														className="text-xs px-2 py-1"
													>
														<code>{permission.permission_name}</code>
													</Badge>
												))}
											</div>
										</div>
									)}
							</div>
						</div>

						<div className="p-4 rounded-xl border bg-muted/50">
							<div className="flex items-start space-x-3">
								<div className="p-2 rounded-lg bg-primary/10">
									<Lock className="h-4 w-4 text-primary" />
								</div>
								<div className="text-sm space-y-1">
									<p className="font-semibold text-foreground">Next Steps:</p>
									<ul className="text-muted-foreground space-y-1 text-xs">
										<li>
											• Store the API key in a secure location (password
											manager, environment variables)
										</li>
										<li>
											• Use the key in your application's authorization headers
										</li>
										<li>• Monitor usage in the API keys dashboard</li>
										<li>• Rotate the key regularly for security</li>
									</ul>
								</div>
							</div>
						</div>
					</div>
				</ScrollArea>

				{/* Fixed Footer */}
				<div className="px-6 py-6 border-t mt-auto">
					<DialogFooter className="p-0">
						<div className="flex justify-between w-full">
							<div />
							<Button type="button" onClick={onClose} className="px-8">
								<Check className="mr-2 h-4 w-4" />
								Done
							</Button>
						</div>
					</DialogFooter>
				</div>
			</div>
		</VerticalStepperStep>
	);
}
