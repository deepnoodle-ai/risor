import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// `risor-source` builds the workspace packages from their TypeScript source.
// A customer's app needs none of this: the published packages are built.
export default defineConfig({
  plugins: [react()],
  resolve: { conditions: ["risor-source"] },
  worker: { format: "es" },
});
