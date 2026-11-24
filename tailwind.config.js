/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    './internal/web/**/*.templ',
    './internal/web/**/*.go',
    './internal/http/**/*.go',
  ],
  theme: {
    extend: {
      fontFamily: {
        mono: ['JetBrains Mono', 'ui-monospace', 'SFMono-Regular', 'monospace'],
      },
      backdropBlur: {
        xs: '2px',
      },
      borderRadius: {
        xs: '0.125rem',
      },
    },
  },
  plugins: [],
};
