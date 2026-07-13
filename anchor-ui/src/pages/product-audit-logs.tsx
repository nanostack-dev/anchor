import { Page } from "@/components/common/Page";
import { AuditLogDatatable } from "@/components/product/audit/AuditLogDatatable";
import { useProduct } from "@/hooks/useProduct";

export default function ProductAuditLogsPage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex items-center justify-center h-64">
					<p className="text-muted-foreground">
						Select a product to view its audit logs.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page>
			<div className="space-y-6">
				<div>
					<h1 className="text-3xl font-bold tracking-tight">Audit Logs</h1>
					<p className="text-muted-foreground">
						Who did what across organizations, workspaces, memberships, API keys
						and roles.
					</p>
				</div>
				<AuditLogDatatable productId={currentProduct.id} />
			</div>
		</Page>
	);
}
