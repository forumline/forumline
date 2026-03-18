import { defineWorkspace } from 'vitest/config'

export default defineWorkspace([
  {
    test: {
      name: 'protocol',
      root: './packages/frontend/protocol',
      include: ['src/**/*.test.ts'],
      exclude: ['node_modules/**', '.claude/**'],
    },
  },
])
