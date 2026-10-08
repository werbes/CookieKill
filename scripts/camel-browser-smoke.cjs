// Real Chrome controls and server-validated movement with an isolated saved herd.
// Uses the same local Playwright/Chrome installation as browser-smoke.cjs.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const net = require('node:net');
const { spawn, spawnSync } = require('node:child_process');
const { chromium } = require(require.resolve('playwright', { paths: [path.resolve(__dirname, '../.tools')] }));

(async () => {
  const root = path.resolve(__dirname, '..');
  const output = path.join(root, 'test-results', `camel-browser-${Date.now()}`);
  const data = path.join(output, 'data');
  fs.mkdirSync(data, { recursive: true });
  const email = 'camel-keeper@example.com';
  const ownerID = 'u_' + crypto.createHash('sha256').update(email).digest('hex').slice(0, 32);
  const slots = count => Array.from({ length: count }, () => ({ item: '', count: 0 }));
  const hotbar = slots(5);
  hotbar[0] = { item: 'sugar', count: 10 };
  const camels = [[0, 20], [-5, 19], [5, 19]].map(([x, z], index) => ({
    id: `${ownerID}_camel_${index + 1}`, x, y: 0, z, yaw: 0,
    island: ownerID, inCave: false, rentalUntil: 0,
  }));
  const profile = {
    id: ownerID, name: 'Camel Keeper', callName: 'Camel Keeper', username: 'camel_keeper', coins: 2000,
    inventoryVersion: 2, safeSlots: slots(3), bagSlots: slots(15), hotbar, selectedSlot: 0,
    levels: {}, friendCode: 'MINT3821', friends: {}, friendRequests: {},
    avatar: { skin: 1, shirt: 'tee', pants: 'trousers', shirtColor: 'teal', pantsColor: 'sand' },
    hats: {}, discovered: { sugar: true }, camelOwned: true, camels,
    homeIsland: { owner: ownerID, visitors: false, builders: {}, chest: {}, objects: [] },
  };
  const playerFile = path.join(data, 'players', ownerID + '.json');
  fs.writeFileSync(path.join(data, 'players.json'), JSON.stringify({ version: 2, profiles: { [ownerID]: profile } }));
  const executable = path.join(output, process.platform === 'win32' ? 'server.exe' : 'server');
  const go = process.env.CK_GO || (process.platform === 'win32' ? 'C:/Program Files/Go/bin/go.exe' : 'go');
  const build = spawnSync(go, ['build', '-o', executable, '.'], { cwd: root, windowsHide: true, encoding: 'utf8' });
  assert.equal(build.status, 0, build.stderr || build.stdout);
  const port = Number(process.env.CK_CAMEL_TEST_PORT) || await new Promise((resolve, reject) => {
    const probe = net.createServer();
    probe.once('error', reject);
    probe.listen(0, '127.0.0.1', () => {
      const selected = probe.address().port;
      probe.close(error => error ? reject(error) : resolve(selected));
    });
  });
  const baseURL = `http://127.0.0.1:${port}`;
  assert.equal(await fetch(baseURL + '/api/world').then(() => true).catch(() => false), false, 'Choose an unused CK_CAMEL_TEST_PORT');
  const source = fs.readFileSync(path.join(root, 'web/state-sync.js'), 'utf8');
  const { createSnapshotDecoder } = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  let serverLog = '', browser, page, snapshot, activeSocket, lastError;
  const errors = [], failed = [], actions = [];
  const server = spawn(executable, ['-dev', '-addr', `127.0.0.1:${port}`, '-data', data], {
    cwd: root, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'],
  });
  server.stdout.on('data', chunk => { serverLog += chunk; });
  server.stderr.on('data', chunk => { serverLog += chunk; });

  async function until(check, message, timeout = 15000) {
    const started = Date.now();
    while (!await check()) {
      assert.ok(Date.now() - started < timeout, `${message}${lastError ? ': ' + lastError : ''}`);
      await new Promise(resolve => setTimeout(resolve, 90));
    }
  }
  const shot = name => page.screenshot({ path: path.join(output, name + '.png') });
  const herdNodes = () => (snapshot.nodes || []).filter(node => node.kind === 'camel' && node.owner === ownerID);
  const mountedRecord = () => snapshot.me.camels.find(camel => camel.id === camels[0].id);
  async function resume() {
    if (await page.locator('#modal').isVisible()) await page.locator('#close-modal').click();
    if (await page.locator('#play-overlay').isVisible()) await page.locator('#play-button').click();
    if (await page.locator('#capture-prompt').isVisible()) await page.locator('#capture-prompt').click();
    await page.waitForFunction(() => !!document.pointerLockElement);
  }
  async function goHome() {
    await resume();
    if (snapshot.me.island === ownerID) return;
    await page.keyboard.press('6');
    await page.getByRole('button', { name: 'Go to my island', exact: true }).click();
    await until(() => snapshot.me.island === ownerID, 'home teleport');
    await resume();
  }
  async function travel(x, z) {
    await resume();
    const started = Date.now();
    try {
      while (Math.hypot(x - snapshot.me.x, z - snapshot.me.z) > .55) {
        assert.ok(Date.now() - started < 30000, `route blocked at ${snapshot.me.x},${snapshot.me.z} toward ${x},${z}`);
        const dx = x - snapshot.me.x, dz = z - snapshot.me.z;
        const divisor = Math.max(4, Math.hypot(dx, dz));
        await page.evaluate(input => { window.__camelMovement = input; }, { x: dx / divisor, z: dz / divisor, sprint: false });
        await page.waitForTimeout(90);
      }
    } finally {
      await page.evaluate(() => { window.__camelMovement = { x: 0, z: 0, sprint: false }; });
    }
    await page.waitForTimeout(180);
  }
  async function face(x, z) {
    const yaw = Math.atan2(-(x - snapshot.me.x), -(z - snapshot.me.z));
    await page.evaluate(({ dx, dy }) => document.dispatchEvent(new MouseEvent('mousemove', { movementX: dx, movementY: dy })), {
      dx: (snapshot.me.yaw - yaw) / .002, dy: snapshot.me.pitch / .002,
    });
    await page.waitForTimeout(180);
  }
  async function mount() {
    await page.keyboard.press('e');
    await until(() => snapshot.me.ridingCamel && snapshot.me.mountedCamelId === camels[0].id, 'E mounts nearby owned camel');
    assert.equal(actions.at(-1).action, 'interact', 'E lets the server choose feeding or mounting from the held item');
    assert.equal(actions.at(-1).target, camels[0].id);
  }
  async function dismount() {
    const cookies = snapshot.me.inventory.sugar, previousActions = actions.length;
    await page.keyboard.press('q');
    await until(() => !snapshot.me.ridingCamel, 'Q dismounts');
    await page.waitForTimeout(180);
    assert.equal(snapshot.me.inventory.sugar, cookies, 'Q while riding must not eat a cookie');
    assert.deepEqual(actions.slice(previousActions).map(action => action.action), ['camel_dismount'], 'Q must only dismount while riding');
  }

  try {
    await until(async () => {
      if (server.exitCode !== null) throw new Error(serverLog);
      return fetch(baseURL + '/api/world').then(response => response.ok).catch(() => false);
    }, 'server startup', 20000);
    browser = await chromium.launch({
      executablePath: process.env.CK_CHROME || (process.platform === 'win32' ? 'C:/Program Files/Google/Chrome/Application/chrome.exe' : undefined),
      headless: true, args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--enable-webgl'],
    });
    const context = await browser.newContext({ viewport: { width: 1366, height: 768 } });
    page = await context.newPage();
    await page.addInitScript(() => {
      const send = WebSocket.prototype.send;
      WebSocket.prototype.send = function (payload) {
        try {
          const message = JSON.parse(payload);
          if (message.type === 'input' && window.__camelMovement) payload = JSON.stringify({ ...message, ...window.__camelMovement });
        } catch {}
        return send.call(this, payload);
      };
    });
    page.on('pageerror', error => errors.push(error.message));
    page.on('response', response => { if (response.status() >= 400) failed.push(`${response.status()} ${response.url()}`); });
    page.on('websocket', socket => {
      activeSocket = socket;
      const decoder = createSnapshotDecoder();
      socket.on('framesent', ({ payload }) => {
        const message = JSON.parse(payload.toString());
        if (message.type === 'action') actions.push(message);
      });
      socket.on('framereceived', ({ payload }) => {
        if (socket !== activeSocket) return;
        try {
          const message = JSON.parse(payload.toString()), next = decoder.apply(message);
          if (next) snapshot = next;
          if (message.type === 'error') { lastError = message.message; errors.push(lastError); }
        } catch (error) { errors.push(error.message); }
      });
    });
    await page.goto(baseURL, { waitUntil: 'networkidle' });
    await page.locator('#email').fill(email);
    await page.locator('#email-submit').click();
    await page.locator('#code-stage').waitFor({ state: 'visible' });
    if (!await page.locator('#login-code').inputValue()) {
      const code = (await page.locator('#development-code').innerText()).match(/\b\d{6}\b/);
      assert.ok(code, 'local login code missing');
      await page.locator('#login-code').fill(code[0]);
    }
    await page.locator('#code-submit').click();
    await page.locator('#start-button').waitFor({ state: 'visible' });
    await until(() => snapshot?.me, 'login snapshot');
    assert.equal(snapshot.me.camels.length, 3, 'saved herd did not load');
    await page.locator('#start-button').click();
    await page.waitForFunction(() => !!document.pointerLockElement);
    await page.keyboard.press('Escape');
    await page.getByRole('button', { name: 'Friends', exact: true }).click();
    const camelNameForm = page.locator(`form[data-camel-name="${camels[0].id}"]`);
    await camelNameForm.locator('input').fill('Saffron');
    await camelNameForm.getByRole('button', { name: 'Save name', exact: true }).click();
    await until(() => snapshot.me.camels[0].name === 'Saffron', 'camel name saved from bottom of Friends');
    assert.equal(snapshot.me.camels[1].name || '', '', 'other camels retain their own names');
    await shot('camel-name-friends');
    await goHome();
    await until(() => herdNodes().length === 3, 'all three parked camels must be visible at home');
    assert.equal(herdNodes().find(node => node.id === camels[0].id).label, 'Saffron');
    await travel(0, 26);
    await face(0, 20);
    await shot('home-herd');
    await travel(0, 21.5);
    await mount();
    assert.equal(herdNodes().length, 2, 'mounted camel must not be duplicated as a parked camel');
    await page.waitForTimeout(200); // Let the interaction hint finish its 120 ms UI refresh.
    await shot('home-riding-camel');
    await dismount();
    await until(() => herdNodes().length === 3, 'Q leaves the camel parked nearby');
    await mount();
    await travel(0, 27);
    await face(0, 28);
    await page.keyboard.press('e');
    await until(() => snapshot.me.island === '' && snapshot.me.inCave, 'return platform reaches the cave');
    assert.equal(snapshot.me.ridingCamel, true, 'return platform dropped the mounted camel');
    assert.equal(snapshot.me.mountedCamelId, camels[0].id);
    assert.equal(mountedRecord().island, '');
    assert.equal(mountedRecord().inCave, true);
    await dismount();
    await until(() => herdNodes().length === 1, 'only the transported camel is parked in the cave');
    await travel(120, -114);
    await face(mountedRecord().x, mountedRecord().z);
    await shot('cave-parked-camel');
    await travel(120, -116);
    await mount();
    await goHome();
    assert.equal(snapshot.me.ridingCamel, true, 'home teleport dropped the mounted camel');
    assert.equal(snapshot.me.mountedCamelId, camels[0].id);
    assert.equal(mountedRecord().island, ownerID);
    assert.equal(mountedRecord().inCave, false);
    await travel(-8, 15);
    await dismount();
    const savedHerd = structuredClone(snapshot.me.camels);
    await until(() => {
      try { return JSON.stringify(JSON.parse(fs.readFileSync(playerFile, 'utf8')).profile.camels) === JSON.stringify(savedHerd); }
      catch { return false; }
    }, 'herd positions persisted to the player file');
    snapshot = null;
    await page.reload({ waitUntil: 'networkidle' });
    await page.locator('#start-button').waitFor({ state: 'visible' });
    await until(() => snapshot?.me, 'reconnected snapshot');
    assert.deepEqual(snapshot.me.camels, savedHerd, 'reconnect lost saved herd or parked positions');
    await page.locator('#start-button').click();
    await page.waitForFunction(() => !!document.pointerLockElement);
    await goHome();
    await until(() => herdNodes().length === 3, 'returning home must show all saved camels');
    await travel(0, 27);
    await face(-2, 18);
    await shot('home-herd-after-travel');
    assert.deepEqual(errors, [], 'browser or server action errors');
    assert.deepEqual(failed, [], 'failed asset/API requests');
    console.log('PASS: three owned camels render; E mounts nearby camel; Q dismounts without eating; mounted camel travels home→cave→home; parked herd and positions persist; no browser errors.');
    console.log('Screenshots: ' + output);
  } catch (error) {
    if (page && !page.isClosed()) await shot('failure').catch(() => {});
    console.error('Camel diagnostics', { errors, failed, lastError, me: snapshot?.me, nodes: snapshot && herdNodes() });
    throw error;
  } finally {
    if (browser) await browser.close();
    server.kill();
    await Promise.race([new Promise(resolve => server.once('exit', resolve)), new Promise(resolve => setTimeout(resolve, 3000))]);
    fs.writeFileSync(path.join(output, 'server.log'), serverLog);
    fs.writeFileSync(path.join(output, 'actions.json'), JSON.stringify(actions, null, 2));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
