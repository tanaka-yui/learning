const Fastify = require('fastify')

function tallyVersions(responses) {
  return responses.reduce((acc, r) => {
    acc[r.version] = (acc[r.version] || 0) + 1
    return acc
  }, {})
}

async function fetchEcho(url) {
  const res = await fetch(url + '/api/v1/echo')
  return res.json()
}

const app = Fastify()
app.get('/healthz', async () => 'ok')
app.get('/', async () => {
  const target = process.env.API_URL || 'http://api'
  const calls = await Promise.all(Array.from({ length: 10 }, () => fetchEcho(target)))
  return tallyVersions(calls)
})

if (require.main === module) {
  app.listen({ host: '0.0.0.0', port: 3000 }).catch((err) => {
    console.error(err)
    process.exit(1)
  })
}

module.exports = { tallyVersions }
