import http from 'node:http'

const host = process.env.HAOHAO_OPENAI_PROXY_HOST ?? '127.0.0.1'
const port = Number(process.env.HAOHAO_OPENAI_PROXY_PORT ?? '11234')
const target = new URL(process.env.HAOHAO_OPENAI_PROXY_TARGET ?? 'http://192.168.1.28:1234')

const server = http.createServer((req, res) => {
  const upstream = http.request({
    hostname: target.hostname,
    port: target.port || 80,
    method: req.method,
    path: req.url,
    headers: {
      ...req.headers,
      host: target.host,
    },
  }, (upstreamRes) => {
    res.writeHead(upstreamRes.statusCode ?? 502, upstreamRes.headers)
    upstreamRes.pipe(res)
  })

  upstream.on('error', (error) => {
    res.writeHead(502, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: error.message }))
  })

  req.pipe(upstream)
})

server.listen(port, host, () => {
  console.log(`OpenAI-compatible proxy listening on http://${host}:${port} -> ${target.origin}`)
})
