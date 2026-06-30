const { test } = require('node:test')
const assert = require('node:assert/strict')
const { tallyVersions } = require('./server')

test('tallyVersions counts v1/v2', () => {
  const result = tallyVersions([{ version: 'v1' }, { version: 'v2' }, { version: 'v1' }])
  assert.deepEqual(result, { v1: 2, v2: 1 })
})
