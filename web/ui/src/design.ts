// Central design tokens — reference CSS variables defined in index.css.
// Update the CSS vars for theming; update these exports for type-safe usage in components.

export const C = {
  bg:      'var(--bg)',
  surface: 'var(--surface)',
  surf2:   'var(--surf2)',
  surf3:   'var(--surf3)',
  border:  'var(--border)',
  accent:  'var(--accent)',
  accHov:  'var(--acc-hov)',
  red:     'var(--red)',
  text:    'var(--text)',
  text2:   'var(--text2)',
  text3:   'var(--text3)',
} as const

export const R = { sm: 6, md: 10, lg: 14, xl: 20, full: 9999 } as const

export const S = { 1: 4, 2: 8, 3: 12, 4: 16, 5: 20, 6: 24, 8: 32, 10: 40, 12: 48 } as const

export const T = {
  xs:    '0.75rem',   // 12px
  sm:    '0.8125rem', // 13px
  base:  '0.9375rem', // 15px
  lg:    '1.0625rem', // 17px
  xl:    '1.25rem',   // 20px
  '2xl': '1.5rem',    // 24px
  '3xl': '1.875rem',  // 30px
} as const
