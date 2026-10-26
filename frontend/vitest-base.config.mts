import { defineConfig } from 'vitest/config';

// Merged by the Angular unit-test builder into its own Vitest settings (angular.json
// runnerConfig). Vitest's five seconds a test are too short for a component test on a shared CI
// runner under coverage (cowork's experience of 2026-10-05); the coverage lands where the CI job
// collects it.
export default defineConfig({
  test: {
    testTimeout: 15_000,
    coverage: {
      reportsDirectory: 'coverage/frontend',
    },
  },
});
