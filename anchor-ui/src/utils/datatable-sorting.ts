export function mapSortingToApiField<T>(columnId: string, defaultField: T): T {
	return (columnId as T) || defaultField;
}
