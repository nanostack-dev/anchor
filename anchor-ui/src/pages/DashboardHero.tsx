type DashboardHeroProps = {
	subtitle: string;
};

/**
 * The dashboard landing header. Split out from `DashboardPage` so the heading
 * itself — not the queries that feed its subtitle — can be storied and
 * screenshotted.
 */
export function DashboardHero({ subtitle }: DashboardHeroProps) {
	return (
		<header className="relative isolate overflow-hidden rounded-3xl border border-border bg-card px-6 py-8 shadow-sm motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:fill-mode-both motion-safe:duration-500">
			<div
				aria-hidden
				className="pointer-events-none absolute -right-20 -top-24 size-64 rounded-full bg-primary/10 blur-3xl"
			/>
			<div
				aria-hidden
				className="pointer-events-none absolute -bottom-24 left-10 size-48 rounded-full bg-chart-2/10 blur-3xl"
			/>
			<h1 className="font-heading text-3xl font-semibold tracking-tight text-foreground">
				Dashboard
			</h1>
			<p className="mt-1.5 max-w-prose text-sm text-muted-foreground">
				{subtitle}
			</p>
		</header>
	);
}
