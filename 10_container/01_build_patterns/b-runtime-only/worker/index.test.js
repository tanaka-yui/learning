const { test } = require('node:test')
const assert = require('node:assert/strict')
const { buildMessage } = require('./index')

test('buildMessage returns lang=node', () => {
  assert.equal(buildMessage('host1').lang, 'node')
  assert.equal(buildMessage('host1').host, 'host1')
})
