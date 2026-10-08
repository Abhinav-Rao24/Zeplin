import type { Config } from "tailwindcss";

const config: Config = {
  content: [
    "./src/pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/components/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/app/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        parchment: "#F6F5EE",
        cardbg: "#FFFFFF",
        cardborder: "#E8E4DA",
        forest: {
          50: "#F2F6F3",
          100: "#E2ECE5",
          200: "#C6DACD",
          500: "#416252",
          600: "#345042",
          700: "#2B4337",
          800: "#22352B",
          900: "#18261F",
        },
        sage: {
          50: "#F4F6F4",
          100: "#E6ECE7",
          200: "#D1DDD3",
          300: "#B8C9BC",
        },
        charcoal: "#1A231F",
        mutedgrey: "#6B756E",
      },
    },
  },
  plugins: [],
};
export default config;
