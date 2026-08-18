UPDATE organization_license_changes
SET change_type = 'MIGRATED'
WHERE change_type = 'SET';
