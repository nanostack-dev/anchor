import { LoginForm } from "@/components/auth/LoginForm";
import { Link } from "@tanstack/react-router";

export function LoginPage() {
	return (
		<div className="grid min-h-svh lg:grid-cols-2">
			<div className="relative flex flex-col items-center justify-center gap-4 p-6 md:p-10">
				<div className="absolute left-6 top-6 md:left-10 md:top-10">
					<Link to="/" className="flex items-center gap-2 font-medium">
						<img src="/logo.svg" alt="Anchor Logo" className="h-8 w-auto" />
						<span className="text-xl font-semibold">Anchor</span>
					</Link>
				</div>
				<div className="flex flex-1 items-center justify-center">
					<LoginForm className="w-full max-w-lg" />
				</div>
			</div>
			<div className="relative hidden bg-muted lg:block">
				<img
					src="/auth/login.jpg"
					alt=""
					className="absolute inset-0 h-full w-full object-cover"
				/>
			</div>
		</div>
	);
}
