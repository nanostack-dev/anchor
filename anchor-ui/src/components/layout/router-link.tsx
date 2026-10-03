import type { LinkComponentProps } from "@nanostackorg/design-system/provider";
import { Link } from "@tanstack/react-router";

export function RouterLink({ href, ref, ...props }: LinkComponentProps) {
	if (
		/^[a-z][a-z0-9+.-]*:/i.test(href) ||
		href.startsWith("//") ||
		href.startsWith("#")
	) {
		return <a ref={ref} href={href} {...props} />;
	}
	const url = new URL(href, "http://router.local");
	return (
		<Link
			ref={ref}
			to={url.pathname}
			search={url.search ? Object.fromEntries(url.searchParams) : undefined}
			hash={url.hash ? url.hash.slice(1) : undefined}
			activeOptions={{ exact: true }}
			{...props}
		/>
	);
}
