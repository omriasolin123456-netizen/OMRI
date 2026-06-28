/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,ts,jsx,tsx}"],
  theme: {
    extend: {
      fontFamily: {
        sans: ["Heebo", "Assistant", "system-ui", "sans-serif"],
        display: ["Assistant", "Heebo", "sans-serif"],
      },
      colors: {
        sand: {
          50: "#fdf9f3",
          100: "#f8edd9",
          200: "#f0d9b0",
        },
        gold: {
          400: "#e9b949",
          500: "#d4a017",
          600: "#b8860b",
        },
        night: {
          800: "#0f1b2d",
          900: "#0a1422",
          950: "#060d18",
        },
        brand: {
          500: "#0ea5a3",
          600: "#0b8a88",
          700: "#076e6c",
        },
      },
      boxShadow: {
        card: "0 10px 40px -12px rgba(15, 27, 45, 0.25)",
        glow: "0 0 0 1px rgba(212,160,23,0.4), 0 18px 50px -12px rgba(212,160,23,0.45)",
      },
      keyframes: {
        "fade-up": {
          "0%": { opacity: "0", transform: "translateY(24px)" },
          "100%": { opacity: "1", transform: "translateY(0)" },
        },
        float: {
          "0%, 100%": { transform: "translateY(0)" },
          "50%": { transform: "translateY(-8px)" },
        },
        pulseRing: {
          "0%": { transform: "scale(0.9)", opacity: "0.7" },
          "70%": { transform: "scale(1.4)", opacity: "0" },
          "100%": { transform: "scale(1.4)", opacity: "0" },
        },
      },
      animation: {
        "fade-up": "fade-up 0.7s ease-out both",
        float: "float 4s ease-in-out infinite",
        "pulse-ring": "pulseRing 2.2s cubic-bezier(0.4,0,0.6,1) infinite",
      },
    },
  },
  plugins: [],
};
