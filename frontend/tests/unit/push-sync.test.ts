/// <reference types="node" />
import { test } from 'node:test';
import assert from 'node:assert/strict';

import { pushSupported, releasePushForLogout, syncSubscription } from '@/lib/push';

test('tanpa browser (Node/Expo Go) push tidak didukung dan tidak melempar', async () => {
  assert.equal(pushSupported(), false);
  assert.equal(await syncSubscription(), false);
  await assert.doesNotReject(releasePushForLogout());
});
