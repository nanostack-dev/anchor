import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@nanostackorg/design-system/components/select";
export interface SmtpFormState {
	host: string;
	port: string;
	encryption: string;
	authMethod: string;
	username: string;
	password: string;
	fromAddress: string;
	fromName: string;
	replyTo: string;
	enabled: boolean;
}

export function SmtpConfigForm({
	form,
	setField,
	errors,
	isNew,
}: {
	form: SmtpFormState;
	setField: <K extends keyof SmtpFormState>(
		key: K,
		value: SmtpFormState[K],
	) => void;
	errors: Partial<Record<keyof SmtpFormState, string>>;
	isNew: boolean;
}) {
	return (
		<div className="flex flex-col gap-4">
			<div className="grid grid-cols-[1fr_120px] gap-3">
				<div className="flex flex-col gap-1">
					<Label htmlFor="smtp-host">Host *</Label>
					<Input
						id="smtp-host"
						placeholder="smtp.protonmail.ch"
						value={form.host}
						onChange={(e) => setField("host", e.target.value)}
						font="mono"
					/>
					{errors.host && (
						<p className="text-xs text-destructive">{errors.host}</p>
					)}
				</div>
				<div className="flex flex-col gap-1">
					<Label htmlFor="smtp-port">Port *</Label>
					<Input
						id="smtp-port"
						placeholder="587"
						value={form.port}
						onChange={(e) => setField("port", e.target.value)}
						font="mono"
					/>
					{errors.port && (
						<p className="text-xs text-destructive">{errors.port}</p>
					)}
				</div>
			</div>

			<div className="grid grid-cols-2 gap-3">
				<div className="flex flex-col gap-1">
					<Label htmlFor="smtp-encryption">Encryption</Label>
					<Select
						items={[
							{ value: "STARTTLS", label: "STARTTLS (587)" },
							{ value: "TLS", label: "Implicit TLS (465)" },
							{ value: "NONE", label: "None (dev only)" },
						]}
						value={form.encryption}
						onValueChange={(v) => setField("encryption", v ?? "")}
					>
						<SelectTrigger id="smtp-encryption" width="fill">
							<SelectValue />
						</SelectTrigger>
						<SelectContent aria-label="Encryption options">
							<SelectItem value="STARTTLS">STARTTLS (587)</SelectItem>
							<SelectItem value="TLS">Implicit TLS (465)</SelectItem>
							<SelectItem value="NONE">None (dev only)</SelectItem>
						</SelectContent>
					</Select>
				</div>
				<div className="flex flex-col gap-1">
					<Label htmlFor="smtp-auth-method">Auth Method</Label>
					<Select
						items={[
							{ value: "PLAIN", label: "PLAIN" },
							{ value: "LOGIN", label: "LOGIN" },
						]}
						value={form.authMethod}
						onValueChange={(v) => setField("authMethod", v ?? "")}
					>
						<SelectTrigger id="smtp-auth-method" width="fill">
							<SelectValue />
						</SelectTrigger>
						<SelectContent aria-label="Authentication method options">
							<SelectItem value="PLAIN">PLAIN</SelectItem>
							<SelectItem value="LOGIN">LOGIN</SelectItem>
						</SelectContent>
					</Select>
				</div>
			</div>

			<div className="flex flex-col gap-1">
				<Label htmlFor="smtp-username">Username *</Label>
				<Input
					id="smtp-username"
					placeholder="you@proton.me"
					value={form.username}
					onChange={(e) => setField("username", e.target.value)}
					font="mono"
				/>
				{errors.username && (
					<p className="text-xs text-destructive">{errors.username}</p>
				)}
			</div>

			<div className="flex flex-col gap-1">
				<Label htmlFor="smtp-password">
					{isNew ? "Password *" : "Password"}
				</Label>
				<Input
					id="smtp-password"
					type="password"
					placeholder={
						isNew
							? "SMTP token or app password"
							: "Leave blank to keep existing"
					}
					value={form.password}
					onChange={(e) => setField("password", e.target.value)}
					font="mono"
				/>
				{errors.password && (
					<p className="text-xs text-destructive">{errors.password}</p>
				)}
			</div>

			<div className="grid grid-cols-2 gap-3">
				<div className="flex flex-col gap-1">
					<Label htmlFor="smtp-from">From Address *</Label>
					<Input
						id="smtp-from"
						placeholder="you@proton.me"
						value={form.fromAddress}
						onChange={(e) => setField("fromAddress", e.target.value)}
						font="mono"
					/>
					{errors.fromAddress && (
						<p className="text-xs text-destructive">{errors.fromAddress}</p>
					)}
				</div>
				<div className="flex flex-col gap-1">
					<Label htmlFor="smtp-from-name">From Name</Label>
					<Input
						id="smtp-from-name"
						placeholder="Your App"
						value={form.fromName}
						onChange={(e) => setField("fromName", e.target.value)}
					/>
				</div>
			</div>

			<div className="flex flex-col gap-1">
				<Label htmlFor="smtp-reply-to">Reply-To</Label>
				<Input
					id="smtp-reply-to"
					placeholder="support@yourapp.com (optional)"
					value={form.replyTo}
					onChange={(e) => setField("replyTo", e.target.value)}
					font="mono"
				/>
			</div>
		</div>
	);
}
