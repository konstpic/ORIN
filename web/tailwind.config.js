/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        sync: {
          synced: "#16a34a",
          outOfSync: "#f97316",
          unknown: "#6b7280",
        },
        health: {
          healthy: "#16a34a",
          progressing: "#0ea5e9",
          degraded: "#dc2626",
          suspended: "#a855f7",
          missing: "#6b7280",
        },
      },
    },
  },
  plugins: [],
};
