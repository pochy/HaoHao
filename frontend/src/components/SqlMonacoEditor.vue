<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import * as monaco from 'monaco-editor/esm/vs/editor/editor.api.js'
import 'monaco-editor/esm/vs/basic-languages/sql/sql.contribution.js'
import EditorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'

interface SqlSchemaColumnHint {
  name: string
  type?: string
}

interface SqlSchemaTableHint {
  database: string
  table: string
  columns: SqlSchemaColumnHint[]
}

interface SqlSchemaHints {
  tables: SqlSchemaTableHint[]
}

type MonacoEditor = monaco.editor.IStandaloneCodeEditor
type MonacoModel = monaco.editor.ITextModel

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  schemaHints: SqlSchemaHints
  disabled?: boolean
}>(), {
  placeholder: '',
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  run: []
}>()

const MONACO_OWNER = 'haohao-dataset-sql'
const CLICKHOUSE_KEYWORDS = [
  'SELECT',
  'FROM',
  'WHERE',
  'GROUP BY',
  'ORDER BY',
  'LIMIT',
  'JOIN',
  'LEFT JOIN',
  'INNER JOIN',
  'ON',
  'AS',
  'WITH',
  'UNION ALL',
  'CREATE TABLE',
  'CREATE TABLE IF NOT EXISTS',
  'INSERT INTO',
  'ENGINE',
  'ORDER BY',
  'count()',
  'sum()',
  'avg()',
  'min()',
  'max()',
  'uniq()',
  'toDate()',
  'toDateTime()',
  'ifNull()',
]

const globalWithMonaco = globalThis as typeof globalThis & {
  MonacoEnvironment?: { getWorker?: () => Worker }
}

globalWithMonaco.MonacoEnvironment = {
  getWorker: () => new EditorWorker(),
}

const editorHost = ref<HTMLElement | null>(null)
const fallbackMode = ref(false)
const editor = shallowRef<MonacoEditor | null>(null)
const model = shallowRef<MonacoModel | null>(null)
const completionProvider = shallowRef<monaco.IDisposable | null>(null)
const resizeObserver = shallowRef<ResizeObserver | null>(null)

let changeListener: monaco.IDisposable | null = null

const schemaTables = computed(() => props.schemaHints.tables.filter((table) => table.database && table.table))
const knownTenantIds = computed(() => {
  const ids = new Set<string>()
  for (const table of schemaTables.value) {
    const match = /^hh_t_(\d+)_(?:raw|work|gold|gold_internal)$/i.exec(table.database)
    if (match) {
      ids.add(match[1])
    }
  }
  return ids
})

watch(
  () => props.modelValue,
  (value) => {
    const currentModel = model.value
    if (!currentModel || currentModel.getValue() === value) {
      return
    }
    currentModel.pushEditOperations(
      [],
      [{ range: currentModel.getFullModelRange(), text: value }],
      () => null,
    )
    validateSql()
  },
)

watch(
  () => props.disabled,
  (disabled) => {
    editor.value?.updateOptions({ readOnly: disabled })
  },
)

watch(schemaTables, () => {
  registerCompletionProvider()
  validateSql()
})

onMounted(async () => {
  await nextTick()
  mountEditor()
})

onBeforeUnmount(() => {
  completionProvider.value?.dispose()
  changeListener?.dispose()
  resizeObserver.value?.disconnect()
  if (model.value) {
    monaco.editor.setModelMarkers(model.value, MONACO_OWNER, [])
  }
  editor.value?.dispose()
  model.value?.dispose()
})

function mountEditor() {
  if (!editorHost.value) {
    fallbackMode.value = true
    return
  }

  try {
    model.value = monaco.editor.createModel(props.modelValue, 'sql')
    editor.value = monaco.editor.create(editorHost.value, {
      model: model.value,
      automaticLayout: true,
      contextmenu: true,
      fontFamily: '"SFMono-Regular", "SF Mono", ui-monospace, "Cascadia Code", "Source Code Pro", monospace',
      fontSize: 13,
      lineHeight: 20,
      lineNumbers: 'on',
      minimap: { enabled: false },
      padding: { top: 12, bottom: 12 },
      placeholder: props.placeholder,
      readOnly: props.disabled,
      roundedSelection: false,
      scrollBeyondLastLine: false,
      scrollbar: {
        verticalScrollbarSize: 10,
        horizontalScrollbarSize: 10,
      },
      tabSize: 2,
      theme: 'vs',
      wordWrap: 'on',
    })

    changeListener = model.value.onDidChangeContent(() => {
      emit('update:modelValue', model.value?.getValue() ?? '')
      validateSql()
    })
    editor.value.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter, () => {
      if ((model.value?.getValue().trim() ?? '') !== '' && !props.disabled) {
        emit('run')
      }
    })
    resizeObserver.value = new ResizeObserver(() => editor.value?.layout())
    resizeObserver.value.observe(editorHost.value)
    registerCompletionProvider()
    validateSql()
  } catch {
    fallbackMode.value = true
  }
}

function registerCompletionProvider() {
  completionProvider.value?.dispose()
  completionProvider.value = monaco.languages.registerCompletionItemProvider('sql', {
    triggerCharacters: ['.', '`', ' ', '\n'],
    provideCompletionItems(currentModel, position) {
      const word = currentModel.getWordUntilPosition(position)
      const range: monaco.IRange = {
        startLineNumber: position.lineNumber,
        endLineNumber: position.lineNumber,
        startColumn: word.startColumn,
        endColumn: word.endColumn,
      }
      const tableContextRange = tableCompletionRange(currentModel, position)
      const tableRange = tableContextRange ?? range
      const isQuotedIdentifierContext = hasOpenIdentifierQuote(currentModel, position)
      const suggestions: monaco.languages.CompletionItem[] = []

      for (const keyword of CLICKHOUSE_KEYWORDS) {
        suggestions.push({
          label: keyword,
          kind: monaco.languages.CompletionItemKind.Keyword,
          insertText: keyword,
          range,
        })
      }

      for (const table of schemaTables.value) {
        const tableName = quotedTableName(table)
        suggestions.push({
          label: tableName,
          kind: monaco.languages.CompletionItemKind.Struct,
          detail: 'ClickHouse table',
          documentation: `${table.columns.length} columns`,
          insertText: tableName,
          range: tableRange,
        })
        suggestions.push({
          label: table.table,
          kind: monaco.languages.CompletionItemKind.Struct,
          detail: table.database,
          documentation: tableName,
          insertText: tableContextRange ? tableName : quoteIdentifier(table.table),
          range: tableRange,
        })
        for (const column of table.columns) {
          suggestions.push({
            label: column.name,
            kind: monaco.languages.CompletionItemKind.Field,
            detail: column.type || 'Column',
            documentation: `${tableName}.${quoteIdentifier(column.name)}`,
            insertText: isQuotedIdentifierContext ? `${column.name.replaceAll('`', '``')}\`` : quoteIdentifier(column.name),
            range,
          })
        }
      }

      return { suggestions }
    },
  })
}

function hasOpenIdentifierQuote(currentModel: MonacoModel, position: monaco.Position) {
  const linePrefix = currentModel.getLineContent(position.lineNumber).slice(0, position.column - 1)
  return /`[^`]*$/.test(linePrefix)
}

function tableCompletionRange(currentModel: MonacoModel, position: monaco.Position): monaco.IRange | null {
  const linePrefix = currentModel.getLineContent(position.lineNumber).slice(0, position.column - 1)
  const match = /(?:`[^`]*`|\w+)(?:\s*\.\s*(?:`[^`]*`|\w*)?)?$/.exec(linePrefix)
  if (!match || match.index === undefined) {
    return null
  }
  const precedingText = linePrefix.slice(0, match.index)
  if (!/(?:\bfrom|\bjoin|\binsert\s+into|\btable)\s*$/i.test(precedingText)) {
    return null
  }
  return {
    startLineNumber: position.lineNumber,
    endLineNumber: position.lineNumber,
    startColumn: match.index + 1,
    endColumn: position.column,
  }
}

function validateSql() {
  const currentModel = model.value
  if (!currentModel) {
    return
  }

  const sql = currentModel.getValue()
  const markers: monaco.editor.IMarkerData[] = []
  const trimmed = sql.trim()

  if (trimmed === '') {
    markers.push(markerForRange(currentModel, 0, Math.max(sql.length, 1), 'SQL statement is required.', monaco.MarkerSeverity.Error))
  }

  if (hasMultipleStatements(sql)) {
    const semicolonIndex = firstStatementTerminator(sql)
    markers.push(markerForRange(
      currentModel,
      semicolonIndex,
      semicolonIndex + 1,
      'Only one SQL statement is allowed.',
      monaco.MarkerSeverity.Error,
    ))
  }

  addForbiddenMarkers(currentModel, markers, sql)
  addUnknownColumnMarkers(currentModel, markers, sql)
  monaco.editor.setModelMarkers(currentModel, MONACO_OWNER, markers)
}

function addForbiddenMarkers(currentModel: MonacoModel, markers: monaco.editor.IMarkerData[], sql: string) {
  const checks: Array<{ pattern: RegExp, message: string }> = [
    { pattern: /(?:^|[^\w`])`?system`?\s*\./gi, message: 'system database is not available.' },
    { pattern: /(?:^|[^\w`])`?default`?\s*\./gi, message: 'default database is not available.' },
    { pattern: /\b`?(file|url|s3|hdfs|azureBlobStorage|gcs)`?\s*\(/gi, message: 'External table functions are disabled.' },
    { pattern: /\bhh_t_(\d+)_gold_internal\b/gi, message: 'gold internal databases are not available.' },
    { pattern: /`hh_t_(\d+)_gold_internal`/gi, message: 'gold internal databases are not available.' },
    { pattern: /\bhh_t_(\d+)_(?:raw|work|gold|gold_internal)\b/gi, message: 'Cross-tenant databases are not available.' },
    { pattern: /`hh_t_(\d+)_(?:raw|work|gold|gold_internal)`/gi, message: 'Cross-tenant databases are not available.' },
  ]

  for (const check of checks) {
    check.pattern.lastIndex = 0
    for (const match of sql.matchAll(check.pattern)) {
      if (match.index === undefined) {
        continue
      }
      if (check.message.startsWith('Cross-tenant') && isKnownTenantId(match[1])) {
        continue
      }
      const start = match.index + (match[0].startsWith('.') ? 1 : 0)
      markers.push(markerForRange(currentModel, start, match.index + match[0].length, check.message, monaco.MarkerSeverity.Error))
    }
  }
}

function addUnknownColumnMarkers(currentModel: MonacoModel, markers: monaco.editor.IMarkerData[], sql: string) {
  const parsed = parseSimpleSelect(sql)
  if (!parsed) {
    return
  }
  const table = findSchemaTable(parsed.table)
  if (!table) {
    return
  }
  const knownColumns = new Set(table.columns.map((column) => normalizeIdentifier(column.name)))
  for (const column of parsed.columns) {
    if (!knownColumns.has(normalizeIdentifier(column.name))) {
      markers.push(markerForRange(
        currentModel,
        column.start,
        column.end,
        `Unknown column for ${quotedTableName(table)}.`,
        monaco.MarkerSeverity.Warning,
      ))
    }
  }
}

function parseSimpleSelect(sql: string): { table: string, columns: Array<{ name: string, start: number, end: number }> } | null {
  const match = /\bselect\b([\s\S]+?)\bfrom\b\s+((?:`[^`]+`|\w+)(?:\s*\.\s*(?:`[^`]+`|\w+))?)/i.exec(sql)
  if (!match || match.index === undefined) {
    return null
  }
  const selectStart = match.index + match[0].toLowerCase().indexOf('select') + 'select'.length
  const selectText = match[1]
  const table = match[2]
  const columns: Array<{ name: string, start: number, end: number }> = []
  let offset = 0

  for (const rawPart of selectText.split(',')) {
    const partStart = selectStart + offset
    offset += rawPart.length + 1
    const trimmedPart = rawPart.trim()
    const leadingWhitespace = rawPart.length - rawPart.trimStart().length
    if (!trimmedPart || trimmedPart === '*' || /[()\s+\-*/]/.test(trimmedPart)) {
      continue
    }
    const withoutAlias = trimmedPart.replace(/\s+as\s+[\w`]+$/i, '').trim()
    const identifier = unquoteIdentifier(withoutAlias.split('.').pop() ?? '')
    if (/^[A-Za-z_][\w$]*$/.test(identifier)) {
      const start = partStart + leadingWhitespace + trimmedPart.lastIndexOf(withoutAlias)
      columns.push({ name: identifier, start, end: start + withoutAlias.length })
    }
  }

  return columns.length > 0 ? { table, columns } : null
}

function markerForRange(
  currentModel: MonacoModel,
  startOffset: number,
  endOffset: number,
  message: string,
  severity: monaco.MarkerSeverity,
): monaco.editor.IMarkerData {
  const start = currentModel.getPositionAt(Math.max(0, startOffset))
  const end = currentModel.getPositionAt(Math.max(startOffset + 1, endOffset))
  return {
    startLineNumber: start.lineNumber,
    startColumn: start.column,
    endLineNumber: end.lineNumber,
    endColumn: end.column,
    message,
    severity,
  }
}

function hasMultipleStatements(sql: string) {
  return firstStatementTerminator(sql) >= 0
}

function firstStatementTerminator(sql: string) {
  let quote: '"' | '\'' | '`' | null = null
  let lineComment = false
  let blockComment = false
  for (let index = 0; index < sql.length; index += 1) {
    const char = sql[index]
    const next = sql[index + 1]
    if (lineComment) {
      if (char === '\n') {
        lineComment = false
      }
      continue
    }
    if (blockComment) {
      if (char === '*' && next === '/') {
        blockComment = false
        index += 1
      }
      continue
    }
    if (quote) {
      if (char === '\\') {
        index += 1
        continue
      }
      if (char === quote) {
        quote = null
      }
      continue
    }
    if (char === '-' && next === '-') {
      lineComment = true
      index += 1
      continue
    }
    if (char === '/' && next === '*') {
      blockComment = true
      index += 1
      continue
    }
    if (char === '"' || char === '\'' || char === '`') {
      quote = char
      continue
    }
    if (char === ';' && sql.slice(index + 1).trim() !== '') {
      return index
    }
  }
  return -1
}

function findSchemaTable(rawTable: string) {
  const normalized = normalizeTableName(rawTable)
  return schemaTables.value.find((table) => (
    normalizeTableName(quotedTableName(table)) === normalized
      || normalizeIdentifier(table.table) === normalized
  ))
}

function quotedTableName(table: SqlSchemaTableHint) {
  return `${quoteIdentifier(table.database)}.${quoteIdentifier(table.table)}`
}

function quoteIdentifier(value: string) {
  return `\`${value.replaceAll('`', '``')}\``
}

function unquoteIdentifier(value: string) {
  const trimmed = value.trim()
  if (trimmed.startsWith('`') && trimmed.endsWith('`')) {
    return trimmed.slice(1, -1).replaceAll('``', '`')
  }
  return trimmed
}

function normalizeTableName(value: string) {
  return value
    .split('.')
    .map((part) => normalizeIdentifier(part))
    .join('.')
}

function normalizeIdentifier(value: string) {
  return unquoteIdentifier(value).toLowerCase()
}

function isKnownTenantId(id: string | undefined) {
  return !id || knownTenantIds.value.size === 0 || knownTenantIds.value.has(id)
}
</script>

<template>
  <div class="dataset-sql-editor">
    <div v-show="!fallbackMode" ref="editorHost" class="dataset-sql-editor-host" />
    <textarea
      v-if="fallbackMode"
      class="field-input textarea-input dataset-sql-input dataset-sql-editor-fallback"
      :disabled="disabled"
      :placeholder="placeholder"
      :value="modelValue"
      spellcheck="false"
      @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value)"
      @keydown.meta.enter.prevent="emit('run')"
      @keydown.ctrl.enter.prevent="emit('run')"
    />
  </div>
</template>
