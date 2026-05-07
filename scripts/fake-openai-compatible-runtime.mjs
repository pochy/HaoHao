import { createHash } from 'node:crypto'
import http from 'node:http'

const host = process.env.HAOHAO_FAKE_OPENAI_HOST ?? '127.0.0.1'
const port = Number(process.env.HAOHAO_FAKE_OPENAI_PORT ?? '11234')
const dimension = Number(process.env.HAOHAO_FAKE_OPENAI_DIMENSION ?? '1024')
const embeddingModel = process.env.HAOHAO_FAKE_OPENAI_EMBEDDING_MODEL ?? 'fake-openai-embedding'
const generationModel = process.env.HAOHAO_FAKE_OPENAI_GENERATION_MODEL ?? 'fake-openai-generation'

function readBody(req) {
  return new Promise((resolve, reject) => {
    const chunks = []
    req.on('data', (chunk) => chunks.push(chunk))
    req.on('end', () => {
      try {
        resolve(chunks.length === 0 ? {} : JSON.parse(Buffer.concat(chunks).toString('utf8')))
      } catch (error) {
        reject(error)
      }
    })
    req.on('error', reject)
  })
}

function writeJSON(res, status, body) {
  res.writeHead(status, { 'Content-Type': 'application/json' })
  res.end(JSON.stringify(body))
}

function hashIndex(token) {
  const digest = createHash('sha256').update(token).digest()
  return digest.readUInt32BE(0) % dimension
}

function tokenize(text) {
  const normalized = String(text ?? '').normalize('NFKC').toLowerCase()
  const tokens = new Set(normalized.match(/[a-z0-9]+|[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}ー]+/gu) ?? [])
  for (const phrase of normalized.match(/[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}ー]{2,}/gu) ?? []) {
    for (let i = 0; i < phrase.length - 1; i += 1) {
      tokens.add(phrase.slice(i, i + 2))
    }
  }
  const synonymGroups = [
    ['支払期限', '振込期限', '支払期日', 'paymentdue', 'amountdue'],
    ['請求書', 'invoice'],
    ['発注書', '注文番号', 'purchaseorder', 'po'],
    ['経費精算', '交通費', '宿泊費'],
    ['契約', '契約書', 'contract'],
    ['登録番号', 'registration'],
  ]
  for (const group of synonymGroups) {
    if (group.some((term) => normalized.includes(term))) {
      for (const term of group) {
        tokens.add(term)
      }
    }
  }
  for (const term of ['青葉商事', '東京部品', '菜の花物流', '港システム', '保守契約', 'saas利用契約', '田中', '佐藤', '買掛運用メモ']) {
    if (normalized.includes(term)) {
      tokens.add(term)
    }
  }
  return [...tokens]
}

function embed(text) {
  const vector = new Array(dimension).fill(0)
  const weighted = tokenize(text).map((token) => ({ token, weight: importantTokenWeight(token) }))
  const hasImportantTokens = weighted.some((item) => item.weight >= 5)
  for (const { token, weight } of weighted) {
    if (hasImportantTokens && weight < 3) {
      continue
    }
    vector[hashIndex(token)] += weight
  }
  const norm = Math.sqrt(vector.reduce((sum, value) => sum + value * value, 0)) || 1
  return vector.map((value) => value / norm)
}

function importantTokenWeight(token) {
  if (/^(支払期限|振込期限|支払期日|paymentdue|amountdue|請求書|invoice|発注書|注文番号|purchaseorder|po|経費精算|交通費|宿泊費|契約|契約書|contract|登録番号|registration)$/.test(token)) {
    return 10
  }
  if (/^(青葉商事|東京部品|菜の花物流|港システム|保守契約|saas利用契約|田中|佐藤|買掛運用メモ)$/.test(token)) {
    return 8
  }
  if (/^(inv|tp|nnb|ms|ctr|exp)$/.test(token) || /^\d{4,}$/.test(token)) {
    return 5
  }
  if (token.length >= 4) {
    return 3
  }
  return 1
}

function answerFromPrompt(prompt) {
  const lines = String(prompt ?? '').split(/\r?\n/)
  const contexts = []
  for (const line of lines) {
    const match = /^\[(c\d+)]\s*(.+)$/.exec(line.trim())
    if (match) {
      contexts.push({ id: match[1], text: match[2] })
    }
  }
  if (contexts.length === 0) {
    return { answer: '', claims: [] }
  }
  const context = contexts[0]
  const claim = context.text.length > 220 ? `${context.text.slice(0, 220)}...` : context.text
  return {
    answer: claim,
    claims: [{ text: claim, citationIds: [context.id] }],
  }
}

const server = http.createServer(async (req, res) => {
  try {
    if (req.method === 'GET' && req.url === '/v1/models') {
      writeJSON(res, 200, {
        object: 'list',
        data: [
          { id: embeddingModel, object: 'model' },
          { id: generationModel, object: 'model' },
        ],
      })
      return
    }
    if (req.method === 'POST' && req.url === '/v1/embeddings') {
      const body = await readBody(req)
      const inputs = Array.isArray(body.input) ? body.input : [body.input]
      writeJSON(res, 200, {
        object: 'list',
        model: body.model ?? embeddingModel,
        data: inputs.map((input, index) => ({
          object: 'embedding',
          index,
          embedding: embed(input),
        })),
      })
      return
    }
    if (req.method === 'POST' && req.url === '/v1/chat/completions') {
      const body = await readBody(req)
      const user = (body.messages ?? []).filter((message) => message.role === 'user').at(-1)?.content ?? ''
      writeJSON(res, 200, {
        id: `fake-chat-${Date.now()}`,
        object: 'chat.completion',
        model: body.model ?? generationModel,
        choices: [
          {
            index: 0,
            finish_reason: 'stop',
            message: {
              role: 'assistant',
              content: JSON.stringify(answerFromPrompt(user)),
            },
          },
        ],
      })
      return
    }
    writeJSON(res, 404, { error: 'not found' })
  } catch (error) {
    writeJSON(res, 500, { error: error.message })
  }
})

server.listen(port, host, () => {
  console.log(`fake OpenAI-compatible runtime listening on http://${host}:${port}`)
})
