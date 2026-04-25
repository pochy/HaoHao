import type { TestInfo } from '@playwright/test'

export function runId(testInfo: TestInfo) {
  return `p9-${Date.now()}-${testInfo.workerIndex}`
}
