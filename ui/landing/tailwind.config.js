const { fontFamily } = require('tailwindcss/defaultTheme')

/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './app/**/*.{js,ts,jsx,tsx,mdx}',
    './components/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['var(--font-inter)', ...fontFamily.sans],
      },
      colors: {
        border: 'hsl(0 0% 92%)',
        background: 'hsl(0 0% 100%)',
        foreground: 'hsl(0 0% 4%)',
        muted: 'hsl(0 0% 96%)',
        'muted-foreground': 'hsl(0 0% 46%)',
        accent: {
          DEFAULT: '#059669',
          foreground: '#ffffff',
        },
        card: {
          DEFAULT: 'hsl(0 0% 100%)',
          foreground: 'hsl(0 0% 4%)',
        },
      },
      borderRadius: {
        DEFAULT: '1rem',
        lg: '1rem',
        md: '0.75rem',
        sm: '0.5rem',
      },
      boxShadow: {
        'card': '0 1px 3px 0 rgb(0 0 0 / 0.04)',
        'card-hover': '0 4px 12px 0 rgb(0 0 0 / 0.06)',
        'nav': '0 1px 0 0 rgb(0 0 0 / 0.06)',
      },
      maxWidth: {
        'page': '1280px',
      },
    },
  },
  plugins: [],
}
