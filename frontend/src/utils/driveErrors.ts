import {
  isRetryableApiError,
  toApiErrorMessage,
  toApiErrorRequestId,
  toApiErrorStatus,
  toApiErrorType,
} from '../api/client'

export type DriveErrorPresentation = {
  title: string
  reason: string
  action: string
  requestId: string
  retryable: boolean
}

export function presentDriveUploadError(error: unknown): DriveErrorPresentation {
  const code = driveErrorCode(error)
  const status = toApiErrorStatus(error)
  const requestId = toApiErrorRequestId(error)
  const retryable = isRetryableApiError(error)

  switch (code) {
    case 'drive.file_too_large':
      return {
        title: 'アップロードできませんでした',
        reason: 'ファイルサイズが上限を超えています。',
        action: '10 MB 以下のファイルを選ぶか、管理者に Drive policy の上限変更を依頼してください。',
        requestId,
        retryable: false,
      }
    case 'drive.file_required':
    case 'drive.filename_required':
    case 'drive.invalid_multipart':
    case 'drive.invalid_input':
      return {
        title: 'アップロードできませんでした',
        reason: '送信されたファイル情報が不足しているか、壊れています。',
        action: 'ファイルを選び直してからもう一度アップロードしてください。',
        requestId,
        retryable: false,
      }
    case 'drive.parent_folder_not_found':
      return {
        title: 'アップロード先が見つかりません',
        reason: '指定されたフォルダが見つかりません。',
        action: 'Drive を更新して、存在するフォルダを選び直してください。',
        requestId,
        retryable: false,
      }
    case 'drive.workspace_not_found':
      return {
        title: 'アップロード先が見つかりません',
        reason: '指定されたワークスペースが見つかりません。',
        action: 'ワークスペースを選び直してからアップロードしてください。',
        requestId,
        retryable: false,
      }
    case 'drive.permission_denied':
    case 'drive.policy_denied':
      return {
        title: 'アップロード権限がありません',
        reason: 'この場所へのアップロードは権限または Drive policy により拒否されました。',
        action: 'アップロード先または権限設定を確認してください。',
        requestId,
        retryable: false,
      }
    case 'drive.quota_exceeded':
      return {
        title: 'アップロードできませんでした',
        reason: 'テナントのファイル容量上限を超えています。',
        action: '不要なファイルを削除するか、管理者に容量上限の変更を依頼してください。',
        requestId,
        retryable: false,
      }
    default:
      return {
        title: status && status >= 500 ? '一時的にアップロードできません' : 'アップロードできませんでした',
        reason: safeDriveReason(error),
        action: retryable
          ? '一時的な問題の可能性があります。少し待ってから再試行してください。'
          : '入力内容とアップロード先を確認してください。',
        requestId,
        retryable,
      }
  }
}

export function presentDriveActionError(error: unknown): string {
  const message = safeDriveReason(error)
  const requestId = toApiErrorRequestId(error)
  return requestId ? `${message} Request ID: ${requestId}` : message
}

function driveErrorCode(error: unknown): string {
  const type = toApiErrorType(error)
  const prefix = 'urn:haohao:error:'
  return type.startsWith(prefix) ? type.slice(prefix.length) : type
}

function safeDriveReason(error: unknown): string {
  const message = toApiErrorMessage(error)
  if (/internal server error/i.test(message)) {
    return 'Drive の処理中に問題が発生しました。'
  }
  return message
}
