const os = require('node:os')

function buildMessage(host) {
  return { lang: 'node', host }
}

if (require.main === module) {
  console.log(JSON.stringify(buildMessage(os.hostname())))
  setTimeout(() => process.exit(0), 5000)
}

module.exports = { buildMessage }
