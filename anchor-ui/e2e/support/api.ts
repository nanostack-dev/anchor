import { type APIRequestContext, expect, request } from "playwright/test";

export class AnchorAPI {
	constructor(readonly context: APIRequestContext) {}
	async get<T>(path: string, status = 200): Promise<T> {
		const response = await this.context.get(path);
		expect(response.status(), `${path}: ${await response.text()}`).toBe(status);
		return response.json() as Promise<T>;
	}
	async post<T>(path: string, data: unknown, status = 201): Promise<T> {
		const response = await this.context.post(path, { data });
		expect(response.status(), `${path}: ${await response.text()}`).toBe(status);
		return response.json() as Promise<T>;
	}
	async put<T>(path: string, data: unknown, status = 200): Promise<T> {
		const response = await this.context.put(path, { data });
		expect(response.status(), `${path}: ${await response.text()}`).toBe(status);
		return response.json() as Promise<T>;
	}
	async remove(path: string) {
		const response = await this.context.delete(path);
		expect([204, 404], `${path}: ${await response.text()}`).toContain(
			response.status(),
		);
	}
}

export async function createAPI(baseURL: string, token?: string) {
	return new AnchorAPI(
		await request.newContext({
			baseURL,
			extraHTTPHeaders: token ? { Authorization: `Bearer ${token}` } : {},
		}),
	);
}
