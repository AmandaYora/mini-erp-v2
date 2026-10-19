import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";

export default tseslint.config(
  { ignores: ["dist/**", "node_modules/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["src/**/*.{ts,tsx}"],
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      // Fetch-on-mount effects are the deliberate data-loading pattern in
      // every list/detail page (sync setLoading + alive-guarded async fetch,
      // stable filter deps — no render cascades). Rewriting 60+ effects to
      // dodge a performance heuristic risks behavior regressions for zero
      // user-visible gain, so this stays a warning, not an error.
      "react-hooks/set-state-in-effect": "warn",
      "react-refresh/only-export-components": "warn",
      "@typescript-eslint/no-explicit-any": "warn",
    },
  },
  // DS v2 (PLAN U3): modul dan app hanya boleh memakai fasad shared/components/ui dan patterns;
  // primitif shadcn (shared/components/primitives) adalah detail implementasi.
  {
    files: ["src/modules/**/*.{ts,tsx}", "src/app/**/*.{ts,tsx}"],
    rules: {
      "no-restricted-imports": [
        "error",
        {
          patterns: [
            {
              group: ["@/shared/components/primitives", "@/shared/components/primitives/*"],
              message: "Impor lewat @/shared/components/ui atau /patterns, bukan primitif shadcn langsung (PLAN U3).",
            },
          ],
        },
      ],
    },
  },
);
