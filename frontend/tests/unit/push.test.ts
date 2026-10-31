/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { urlBase64ToBytes } from '@/lib/push';

test('decodes base64url without padding', () => {
  assert.deepEqual(Array.from(urlBase64ToBytes('AQID')), [1, 2, 3]);
});

test('handles url-safe characters', () => {
  assert.deepEqual(Array.from(urlBase64ToBytes('-_8')), [251, 255]);
});
