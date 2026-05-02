<script setup lang="ts">
import { useI18n } from 'vue-i18n'

import { useTenantAdminDetailContext } from '../../tenant-admin/detail-context'

const { t } = useI18n()
const {
  commonStore,
  driveAdminContentAccessMode,
  driveAiEnabled,
  driveAiTrainingOptOut,
  driveAllowedDomains,
  driveAllowedRegions,
  driveAnonymousEditorLinkMaxTTLMinutes,
  driveAnonymousEditorLinksEnabled,
  driveAnonymousEditorLinksRequirePassword,
  driveBlockDownloadUntilScanComplete,
  driveBlockShareUntilScanComplete,
  driveBlockedDomains,
  driveCleanRoomEnabled,
  driveCleanRoomRawExportEnabled,
  driveCmkEnabled,
  driveCollaborationEnabled,
  driveContentScanEnabled,
  driveDataResidencyEnabled,
  driveDlpEnabled,
  driveE2eeEnabled,
  driveE2eeZeroKnowledgeRequired,
  driveEDiscoveryProviderExportEnabled,
  driveEncryptionMode,
  driveExternalDownloadEnabled,
  driveExternalSharingEnabled,
  driveHsmEnabled,
  driveLegalDiscoveryEnabled,
  driveLocalSearchEmbeddingRuntime,
  driveLocalSearchVectorEnabled,
  driveM2MApiEnabled,
  driveMarketplaceEnabled,
  driveMaxFileSizeBytes,
  driveMaxLinkTTLHours,
  driveMaxPublicLinkCount,
  driveMaxWorkspaceCount,
  driveMobileOfflineEnabled,
  driveOfficeCoauthoringEnabled,
  driveOfflineCacheAllowed,
  driveOcrEnabled,
  driveOcrEngine,
  driveOcrLanguages,
  driveOcrMaxPages,
  driveOcrTimeoutSecondsPerPage,
  driveLMStudioBaseURL,
  driveLMStudioModel,
  driveOllamaBaseURL,
  driveOllamaModel,
  driveOnPremGatewayEnabled,
  drivePasswordLinksEnabled,
  drivePlanCode,
  drivePolicyRows,
  drivePrimaryRegion,
  drivePublicLinksEnabled,
  driveRequireApproval,
  driveRequireLinkPassword,
  driveRulesCandidateScoreThreshold,
  driveRulesConfigVisible,
  driveRulesContextWindowRunes,
  driveRulesMaxBlockRunes,
  driveRulesPriceExtractionEnabled,
  driveSearchEnabled,
  driveStructuredExtractionEnabled,
  driveStructuredExtractor,
  driveSyncEnabled,
  driveViewerDownloadEnabled,
  driveOllamaConfigVisible,
  driveLMStudioConfigVisible,
  latestDriveLocalSearchJob,
  latestDriveLocalSearchJobLabel,
  saveCommonSettings,
  store,
} = useTenantAdminDetailContext()
</script>

<template>
  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.sections.drivePolicy') }}</span>
        <h2>{{ t('tenantAdmin.headings.driveAuthorization') }}</h2>
      </div>
    </div>

    <form class="admin-form" @submit.prevent="saveCommonSettings">
      <label class="checkbox-field">
        <input v-model="drivePublicLinksEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.publicLinksEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveExternalSharingEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.externalUserSharingEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveRequireApproval" type="checkbox">
        <span>{{ t('tenantAdmin.fields.externalShareApprovalRequired') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="drivePasswordLinksEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.passwordProtectedLinksEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveRequireLinkPassword" type="checkbox">
        <span>{{ t('tenantAdmin.fields.requireShareLinkPassword') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveViewerDownloadEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.viewerDownloadEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveExternalDownloadEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.externalDownloadEnabled') }}</span>
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.adminContentAccess') }}</span>
        <select v-model="driveAdminContentAccessMode" class="field-input">
          <option value="disabled">{{ t('common.disabled') }}</option>
          <option value="break_glass">{{ t('tenantAdmin.options.breakGlass') }}</option>
        </select>
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.drivePlan') }}</span>
        <select v-model="drivePlanCode" class="field-input">
          <option value="free">{{ t('tenantAdmin.options.free') }}</option>
          <option value="standard">{{ t('tenantAdmin.options.standard') }}</option>
          <option value="enterprise">{{ t('tenantAdmin.options.enterprise') }}</option>
        </select>
      </label>
      <label class="checkbox-field">
        <input v-model="driveAnonymousEditorLinksEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.anonymousEditorLinksEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveAnonymousEditorLinksRequirePassword" type="checkbox">
        <span>{{ t('tenantAdmin.fields.anonymousEditorLinksRequirePassword') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveContentScanEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.contentScanEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveDlpEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.dlpEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveBlockDownloadUntilScanComplete" type="checkbox">
        <span>{{ t('tenantAdmin.fields.blockDownloadUntilScanComplete') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveBlockShareUntilScanComplete" type="checkbox">
        <span>{{ t('tenantAdmin.fields.blockShareUntilScanComplete') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveM2MApiEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.m2mDriveApiEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveSearchEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.searchIndexEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveCollaborationEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.collaborationEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveSyncEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.desktopSyncEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveMobileOfflineEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.mobileOfflineEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveOfflineCacheAllowed" type="checkbox">
        <span>{{ t('tenantAdmin.fields.offlineCacheAllowed') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveCmkEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.customerManagedKeysEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveDataResidencyEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.dataResidencyEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveLegalDiscoveryEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.legalDiscoveryEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveCleanRoomEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.cleanRoomEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveCleanRoomRawExportEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.cleanRoomRawExportEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveOfficeCoauthoringEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.officeCoauthoringEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveEDiscoveryProviderExportEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.eDiscoveryProviderExportEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveHsmEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.dedicatedHsmEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveOnPremGatewayEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.onPremGatewayEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveE2eeEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.e2eeZeroKnowledgeEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveE2eeZeroKnowledgeRequired" type="checkbox">
        <span>{{ t('tenantAdmin.fields.e2eeZeroKnowledgeRequired') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveAiEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.aiClassificationEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveAiTrainingOptOut" type="checkbox">
        <span>{{ t('tenantAdmin.fields.aiProviderTrainingOptOut') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveMarketplaceEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.driveMarketplaceEnabled') }}</span>
      </label>
      <label class="checkbox-field">
        <input v-model="driveOcrEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.ocrEnabled') }}</span>
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.ocrEngine') }}</span>
        <select v-model="driveOcrEngine" class="field-input">
          <option value="tesseract">Tesseract</option>
          <option value="docling">Docling</option>
          <option value="paddleocr">PaddleOCR</option>
        </select>
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.ocrLanguages') }}</span>
        <input v-model="driveOcrLanguages" class="field-input" autocomplete="off" placeholder="jpn, eng">
      </label>
      <label class="checkbox-field">
        <input v-model="driveStructuredExtractionEnabled" type="checkbox">
        <span>{{ t('tenantAdmin.fields.structuredExtractionEnabled') }}</span>
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.structuredExtractor') }}</span>
        <select v-model="driveStructuredExtractor" class="field-input">
          <optgroup label="Local rules">
            <option value="rules">Rules (IDEA, no LLM)</option>
          </optgroup>
          <optgroup label="Non-LLM Python">
            <option value="python">Python helper</option>
            <option value="ginza">GiNZA</option>
            <option value="sudachipy">SudachiPy</option>
          </optgroup>
          <optgroup label="Local LLM">
            <option value="ollama">Ollama</option>
            <option value="lmstudio">LM Studio</option>
          </optgroup>
          <optgroup label="Agent CLI">
            <option value="gemini">Gemini CLI</option>
            <option value="codex">Codex CLI</option>
            <option value="claude">Claude CLI</option>
          </optgroup>
          <optgroup label="Compatibility">
            <option value="docling">Docling</option>
          </optgroup>
        </select>
      </label>
      <template v-if="driveRulesConfigVisible">
        <label class="field">
          <span class="field-label">{{ t('tenantAdmin.fields.rulesCandidateScoreThreshold') }}</span>
          <input v-model.number="driveRulesCandidateScoreThreshold" class="field-input" min="0" max="20" type="number">
        </label>
        <label class="field">
          <span class="field-label">{{ t('tenantAdmin.fields.rulesMaxBlockRunes') }}</span>
          <input v-model.number="driveRulesMaxBlockRunes" class="field-input" min="500" max="10000" type="number">
        </label>
        <label class="field">
          <span class="field-label">{{ t('tenantAdmin.fields.rulesContextWindowRunes') }}</span>
          <input v-model.number="driveRulesContextWindowRunes" class="field-input" min="100" max="3000" type="number">
        </label>
        <label class="checkbox-field">
          <input v-model="driveRulesPriceExtractionEnabled" type="checkbox">
          <span>{{ t('tenantAdmin.fields.rulesPriceExtractionEnabled') }}</span>
        </label>
      </template>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.ocrMaxPages') }}</span>
        <input v-model.number="driveOcrMaxPages" class="field-input" min="1" max="200" type="number">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.ocrTimeoutSecondsPerPage') }}</span>
        <input v-model.number="driveOcrTimeoutSecondsPerPage" class="field-input" min="1" max="300" type="number">
      </label>
      <label v-if="driveOllamaConfigVisible" class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.ollamaBaseUrl') }}</span>
        <input v-model="driveOllamaBaseURL" class="field-input" autocomplete="off" placeholder="http://127.0.0.1:11434">
      </label>
      <label v-if="driveOllamaConfigVisible" class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.ollamaModel') }}</span>
        <input v-model="driveOllamaModel" class="field-input" autocomplete="off" placeholder="llama3.1">
      </label>
      <label v-if="driveLMStudioConfigVisible" class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.lmStudioBaseUrl') }}</span>
        <input v-model="driveLMStudioBaseURL" class="field-input" autocomplete="off" placeholder="http://127.0.0.1:1234">
      </label>
      <label v-if="driveLMStudioConfigVisible" class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.lmStudioModel') }}</span>
        <input v-model="driveLMStudioModel" class="field-input" autocomplete="off" placeholder="local-model">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.encryptionMode') }}</span>
        <select v-model="driveEncryptionMode" class="field-input">
          <option value="service_managed">{{ t('tenantAdmin.options.serviceManaged') }}</option>
          <option value="tenant_managed">{{ t('tenantAdmin.options.tenantManaged') }}</option>
        </select>
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.primaryRegion') }}</span>
        <input v-model="drivePrimaryRegion" class="field-input" autocomplete="off" placeholder="ap-northeast-1">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.allowedRegions') }}</span>
        <input v-model="driveAllowedRegions" class="field-input" autocomplete="off" placeholder="ap-northeast-1, us-east-1">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.maxLinkTtlHours') }}</span>
        <input v-model.number="driveMaxLinkTTLHours" class="field-input" min="1" max="2160" type="number">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.anonymousEditorTtlMinutes') }}</span>
        <input v-model.number="driveAnonymousEditorLinkMaxTTLMinutes" class="field-input" min="1" max="1440" type="number">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.maxFileSizeBytes') }}</span>
        <input v-model.number="driveMaxFileSizeBytes" class="field-input" min="1" type="number">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.maxWorkspaces') }}</span>
        <input v-model.number="driveMaxWorkspaceCount" class="field-input" min="1" max="1000" type="number">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.maxPublicLinks') }}</span>
        <input v-model.number="driveMaxPublicLinkCount" class="field-input" min="1" max="100000" type="number">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.allowedExternalDomains') }}</span>
        <input v-model="driveAllowedDomains" class="field-input" autocomplete="off" placeholder="example.com, partner.example">
      </label>
      <label class="field">
        <span class="field-label">{{ t('tenantAdmin.fields.blockedExternalDomains') }}</span>
        <input v-model="driveBlockedDomains" class="field-input" autocomplete="off" placeholder="blocked.example">
      </label>
      <div class="action-row form-span">
        <button class="primary-button" :disabled="commonStore.saving" type="submit">
          {{ t('tenantAdmin.actions.saveDrivePolicy') }}
        </button>
      </div>
    </form>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.sections.drivePolicy') }}</span>
        <h2>{{ t('tenantAdmin.headings.currentPolicy') }}</h2>
      </div>
    </div>

    <div class="admin-table">
      <table>
        <thead>
          <tr>
            <th scope="col">{{ t('tenantAdmin.fields.policy') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.currentPhase') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="row in drivePolicyRows" :key="row[0]">
            <td>{{ row[0] }}</td>
            <td>{{ row[1] }}</td>
          </tr>
        </tbody>
      </table>
    </div>

    <p class="cell-subtle">
      {{ t('tenantAdmin.policy.auditNotePrefix') }}
      <code>drive.file.*</code>, <code>drive.folder.*</code>, <code>drive.share.*</code>, <code>drive.share_link.*</code>, <code>drive.ocr.*</code>, <code>drive.authz.denied</code>
      {{ t('tenantAdmin.policy.auditNoteSuffix') }}
    </p>
  </section>

  <section class="panel stack">
    <div class="section-header">
      <div>
        <span class="status-pill">{{ t('tenantAdmin.sections.drivePolicy') }}</span>
        <h2>{{ t('tenantAdmin.headings.ocrRuntime') }}</h2>
      </div>
    </div>

    <div class="admin-table">
      <table>
        <tbody>
          <tr>
            <td>{{ t('tenantAdmin.fields.ocrEnabled') }}</td>
            <td>{{ store.driveOCRStatus?.enabled ? t('common.enabled') : t('common.disabled') }}</td>
          </tr>
          <tr>
            <td>{{ t('tenantAdmin.fields.ocrEngine') }}</td>
            <td>{{ store.driveOCRStatus?.ocrEngine ?? '-' }}</td>
          </tr>
          <tr>
            <td>{{ t('tenantAdmin.fields.structuredExtractor') }}</td>
            <td>{{ store.driveOCRStatus?.structuredExtractor ?? '-' }}</td>
          </tr>
          <tr>
            <td>{{ t('tenantAdmin.fields.ollamaModel') }}</td>
            <td>
              {{ store.driveOCRStatus?.ollama.configured ? t('common.enabled') : t('common.disabled') }}
              / {{ store.driveOCRStatus?.ollama.reachable ? t('tenantAdmin.status.available') : t('tenantAdmin.status.unavailable') }}
            </td>
          </tr>
          <tr>
            <td>{{ t('tenantAdmin.fields.lmStudioModel') }}</td>
            <td>
              {{ store.driveOCRStatus?.lmStudio?.configured ? t('common.enabled') : t('common.disabled') }}
              / {{ store.driveOCRStatus?.lmStudio?.reachable ? t('tenantAdmin.status.available') : t('tenantAdmin.status.unavailable') }}
            </td>
          </tr>
          <tr>
            <td>{{ t('tenantAdmin.fields.localSearchVector') }}</td>
            <td>
              {{ driveLocalSearchVectorEnabled ? t('common.enabled') : t('common.disabled') }}
              / {{ t('tenantAdmin.fields.localSearchRuntime') }}: {{ driveLocalSearchEmbeddingRuntime || 'none' }}
            </td>
          </tr>
          <tr>
            <td>{{ t('tenantAdmin.fields.localSearchLatestJob') }}</td>
            <td>
              {{ latestDriveLocalSearchJobLabel }}
              <span v-if="latestDriveLocalSearchJob?.lastError" class="cell-subtle">/ {{ latestDriveLocalSearchJob.lastError }}</span>
            </td>
          </tr>
          <tr v-for="command in store.driveOCRStatus?.localCommands ?? []" :key="command.name">
            <td>{{ t('tenantAdmin.fields.localRuntime') }}: {{ command.name }}</td>
            <td>
              {{ command.configured ? t('common.enabled') : t('common.disabled') }}
              / {{ command.available ? t('tenantAdmin.status.available') : t('tenantAdmin.status.unavailable') }}
              <span v-if="command.version">/ {{ command.version }}</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="admin-table">
      <table>
        <thead>
          <tr>
            <th scope="col">{{ t('tenantAdmin.fields.dependency') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.state') }}</th>
            <th scope="col">{{ t('tenantAdmin.fields.version') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="dependency in store.driveOCRStatus?.dependencies ?? []" :key="dependency.name">
            <td>{{ dependency.name }}</td>
            <td>{{ dependency.available ? t('tenantAdmin.status.available') : t('tenantAdmin.status.unavailable') }}</td>
            <td>{{ dependency.version ?? '-' }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
