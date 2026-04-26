import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: {
      "@": `${import.meta.dirname}/src`,
      "@backend": `${import.meta.dirname}/frontend/bindings/github.com/Jordan-Kowal/grove/backend`,
    },
  },
  test: {
    environment: "node",
    include: ["src/**/*.test.ts"],
  },
});
