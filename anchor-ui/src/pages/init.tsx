import { AuthScreen } from "@/components/auth/AuthScreen";
import { SignupForm } from "@/components/auth/SignupForm";
export function InitPage() {
	return (
		<AuthScreen>
			<SignupForm
				variant="init"
				title="Welcome to Anchor"
				description="Set up your organization and administrator account to get started."
				showLoginLink={false}
			/>
		</AuthScreen>
	);
}
