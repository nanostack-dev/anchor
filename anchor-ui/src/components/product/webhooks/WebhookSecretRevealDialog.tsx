import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { AlertTriangle, Check, Copy, Eye, EyeOff } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

export type SecretRevealVariant = "created" | "rotated";

const copy: Record<
	SecretRevealVariant,
	{ title: string; description: string; extra?: string }
> = {
	created: {
		title: "Copy your signing secret now",
		description:
			"This is the only time Anchor will ever show this secret. It is encrypted at rest and cannot be recovered — if you lose it you must rotate the endpoint and update your receiver.",
	},
	rotated: {
		title: "Copy your new signing secret now",
		description:
			"This is the only time Anchor will ever show this secret. It is encrypted at rest and cannot be recovered — if you lose it you must rotate again.",
		extra:
			"The previous secret keeps signing alongside this one for 24 hours. Both signatures ride in the space-delimited `webhook-signature` header, so you can roll your receiver over without downtime.",
	},
};

interface WebhookSecretRevealDialogProps {
	open: boolean;
	/** Plaintext secret from the create or rotate response, or null while closed. */
	secret: string | null;
	endpointUrl?: string;
	variant: SecretRevealVariant;
	onAcknowledged: () => void;
}

/**
 * One-time reveal of a webhook signing secret.
 *
 * The dialog is deliberately hostile to being dismissed: no close button, no
 * outside-press or escape dismissal, and the only exit requires ticking an
 * explicit acknowledgement. A secret silently lost behind a stray click is
 * unrecoverable, and the customer only finds out when signature verification
 * starts failing in production.
 */
export function WebhookSecretRevealDialog({
	open,
	secret,
	endpointUrl,
	variant,
	onAcknowledged,
}: WebhookSecretRevealDialogProps) {
	const [revealed, setRevealed] = useState(false);
	const [copied, setCopied] = useState(false);
	const [acknowledged, setAcknowledged] = useState(false);

	useEffect(() => {
		if (open) {
			setRevealed(false);
			setCopied(false);
			setAcknowledged(false);
		}
	}, [open]);

	const handleCopy = async () => {
		if (!secret) return;
		try {
			await navigator.clipboard.writeText(secret);
			setCopied(true);
			toast.success("Signing secret copied to clipboard");
			setTimeout(() => setCopied(false), 2000);
		} catch {
			toast.error("Failed to copy the signing secret");
		}
	};

	const text = copy[variant];

	return (
		<Dialog
			open={open}
			disablePointerDismissal
			onOpenChange={(nextOpen) => {
				// Ignore every close attempt until the secret is acknowledged.
				if (!nextOpen && acknowledged) {
					onAcknowledged();
				}
			}}
		>
			<DialogContent className="sm:max-w-[640px]" showCloseButton={false}>
				<DialogHeader>
					<DialogTitle className="flex items-center gap-2 text-warning">
						<AlertTriangle className="size-4" />
						{text.title}
					</DialogTitle>
					<DialogDescription>{text.description}</DialogDescription>
				</DialogHeader>

				<div className="flex flex-col gap-3">
					{endpointUrl && (
						<p className="text-sm text-muted-foreground">
							Endpoint <span className="font-mono">{endpointUrl}</span>
						</p>
					)}

					<div className="flex flex-col gap-2 sm:flex-row sm:items-center">
						<div className="min-w-0 flex-1 rounded-lg border border-border bg-muted p-3 font-mono text-sm break-all">
							{revealed ? (secret ?? "") : "•".repeat(48)}
						</div>
						<div className="flex gap-2">
							<Button
								type="button"
								variant="outline"
								size="icon"
								className="shrink-0"
								onClick={() => setRevealed((value) => !value)}
								aria-label={revealed ? "Hide secret" : "Show secret"}
							>
								{revealed ? (
									<EyeOff className="size-4" />
								) : (
									<Eye className="size-4" />
								)}
							</Button>
							<Button
								type="button"
								variant="outline"
								size="icon"
								className="shrink-0"
								onClick={handleCopy}
								aria-label="Copy secret"
							>
								{copied ? (
									<Check className="size-4 text-success" />
								) : (
									<Copy className="size-4" />
								)}
							</Button>
						</div>
					</div>

					<div className="rounded-lg border border-warning/30 bg-warning/10 p-3">
						<p className="text-sm text-warning">
							Signatures are HMAC-SHA256 over{" "}
							<span className="font-mono">
								{"{webhook-id}.{webhook-timestamp}.{body}"}
							</span>{" "}
							keyed with this value exactly as shown, and delivered in the{" "}
							<span className="font-mono">webhook-signature</span> header as{" "}
							<span className="font-mono">v1,&lt;base64&gt;</span>.
						</p>
						{text.extra && (
							<p className="mt-2 text-sm text-warning">{text.extra}</p>
						)}
					</div>

					<div className="flex items-start gap-2">
						<Checkbox
							id="webhook-secret-acknowledged"
							checked={acknowledged}
							onCheckedChange={(checked) => setAcknowledged(checked === true)}
							className="mt-0.5"
						/>
						<Label
							htmlFor="webhook-secret-acknowledged"
							className="cursor-pointer text-sm font-normal"
						>
							I have copied this secret and stored it somewhere secure. I
							understand it will never be shown again.
						</Label>
					</div>
				</div>

				<DialogFooter>
					<Button
						type="button"
						disabled={!acknowledged}
						onClick={onAcknowledged}
					>
						Done
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
