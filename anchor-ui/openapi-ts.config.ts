import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../apps/anchor/cmd/http/openapi.yaml",
  output: "src/client",
  plugins: [
    {
      name: "zod",
      exportFromIndex: true,
    },
    {
      name: "@hey-api/typescript",
      exportFromIndex: true,
      enums: "typescript",
    },
    {
      name: "@hey-api/client-fetch",
      exportFromIndex: true,
    },
    {
      name: "@tanstack/react-query",
      exportFromIndex: true,
      "~hooks": {
        operations: {
          getKind: (op) => {
            if (op.method === "post" && op.path.includes("/search")) {
              return ["query"] as const;
            }

            return [op.method === "get" ? "query" : "mutation"] as const;
          },
        },
      },
    },
  ],
});
