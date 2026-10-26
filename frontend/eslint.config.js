// @ts-check
const eslint = require('@eslint/js');
const { defineConfig } = require('eslint/config');
const tseslint = require('typescript-eslint');
const angular = require('angular-eslint');

module.exports = defineConfig([
  // The generated API client is never edited, so it is not linted.
  { ignores: ['src/app/api/**', 'public/**'] },
  {
    files: ['**/*.ts'],
    extends: [
      eslint.configs.recommended,
      tseslint.configs.recommended,
      tseslint.configs.stylistic,
      angular.configs.tsRecommended,
    ],
    processor: angular.processInlineTemplates,
    rules: {
      '@angular-eslint/directive-selector': [
        'error',
        { type: 'attribute', prefix: 'app', style: 'camelCase' },
      ],
      '@angular-eslint/component-selector': [
        'error',
        { type: 'element', prefix: 'app', style: 'kebab-case' },
      ],
      // Every generated function assigns its PATH, a side effect that keeps a module in the bundle
      // once anything reaches it: through the barrel, an eager service would pull every operation
      // of the API into the initial bundle (cowork's ADR 0052 D6). Import the one file.
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              regex: '(^|/)api/functions$',
              message: 'Import the operation from api/fn/<tag>/<operation> instead of the barrel.',
            },
          ],
        },
      ],
    },
  },
  {
    files: ['**/*.html'],
    extends: [angular.configs.templateRecommended, angular.configs.templateAccessibility],
    rules: {},
  },
]);
