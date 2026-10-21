import js from '@eslint/js';
import globals from 'globals';

export default [
  { ignores: ['dist/', 'node_modules/', 'test-results/', 'playwright-report/'] },
  js.configs.recommended,
  {
    files: ['scripts/**/*.mjs', 'src/lib/**/*.js', 'tests/**/*.js', '*.js', '*.mjs'],
    languageOptions: { globals: globals.node },
  },
  { files: ['src/client/**/*.js'], languageOptions: { globals: globals.browser } },
  { files: ['src/sw.js'], languageOptions: { globals: globals.serviceworker } },
  {
    files: ['tests/e2e/**/*.js'],
    languageOptions: { globals: { ...globals.node, ...globals.browser } },
  },
  {
    rules: {
      eqeqeq: ['error', 'smart'],
      'no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
      'no-console': 'off',
    },
  },
];
