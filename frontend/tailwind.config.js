/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        background: "rgb(var(--color-bg) / <alpha-value>)",
        foreground: "rgb(var(--color-text) / <alpha-value>)",
        muted: "rgb(var(--color-surface) / <alpha-value>)",
        "muted-foreground": "rgb(var(--color-muted) / <alpha-value>)",
        border: "rgb(var(--color-line) / <alpha-value>)",
        accent: "rgb(var(--color-surface) / <alpha-value>)",
        "accent-foreground": "rgb(var(--color-text) / <alpha-value>)",
        primary: "rgb(var(--color-accent) / <alpha-value>)",
        "primary-foreground": "rgb(var(--color-text) / <alpha-value>)",
        destructive: "239 68 68",
        app: {
          bg: "rgb(var(--color-bg) / <alpha-value>)",
          sidebar: "rgb(var(--color-sidebar) / <alpha-value>)",
          panel: "rgb(var(--color-panel) / <alpha-value>)",
          surface: "rgb(var(--color-surface) / <alpha-value>)",
          text: "rgb(var(--color-text) / <alpha-value>)",
          muted: "rgb(var(--color-muted) / <alpha-value>)",
          line: "rgb(var(--color-line) / <alpha-value>)",
          accent: "rgb(var(--color-accent) / <alpha-value>)",
          accentSoft: "rgb(var(--color-accent-soft) / <alpha-value>)",
        },
      },
      boxShadow: {
        soft: "0 1px 1px rgb(64 45 34 / 0.05), 0 12px 28px rgb(64 45 34 / 0.08)",
      },
    },
  },
  plugins: [],
};
