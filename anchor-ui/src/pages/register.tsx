import { AuthScreen } from "@/components/auth/AuthScreen";
import { SignupForm } from "@/components/auth/SignupForm";
import { registerRoute } from "@/routes/platform/register";
import { useSearch } from "@tanstack/react-router";
export function RegisterPage() {
	const queryParam = useSearch({ from: registerRoute.id });
	return (
		<AuthScreen>
			<SignupForm
				variant="register"
				email={queryParam.email}
				tenantId={queryParam.tenantId}
				invitationCode={queryParam.invitationCode}
			/>
		</AuthScreen>
	);
}
