import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

/**
 * Minimal TanStack Query context for stories.
 *
 * Any component calling `useMutation`/`useQuery` needs a client in scope, even
 * when the story never fires the request. Retries are off so a story that *does*
 * reach the network fails once and settles, instead of holding the play function
 * open through three backoffs until the test times out.
 *
 * A fresh client per render keeps stories isolated — no cache carries from one
 * story file into the next.
 */
export function StoryQuery({ children }: { children: ReactNode }) {
	const queryClient = new QueryClient({
		defaultOptions: {
			queries: { retry: false },
			mutations: { retry: false },
		},
	});

	return (
		<QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
	);
}
