-- Marks the resources the workspace mirror wrote (ADR-0022 A6, phase 2): each
-- mirrors the built-in workspace whose ID is its external_id. A marked
-- resource counts only while that workspace exists, so a workspace an earlier
-- release deleted without its resource stays deleted. A resource the Product
-- registered itself carries no mark. Dropped with the workspaces table (G6).
CREATE TABLE workspace_resources (
    resource_id VARCHAR(255) PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE
);
