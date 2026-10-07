// Isolated Chrome regression: real drink actions and walked distance, no state cheats.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const net = require('node:net');
const {spawn, spawnSync} = require('node:child_process');
const {chromium} = require(require.resolve('playwright', {paths: [path.resolve(__dirname, '../.tools')]}));

(async () => {
  const root = path.resolve(__dirname, '..');
  const output = path.join(root, 'test-results', 'fall-browser-' + Date.now());
  const data = path.join(output, 'data');
  fs.mkdirSync(data, {recursive: true});
  const email = 'fall-review@example.com';
  const id = 'u_' + crypto.createHash('sha256').update(email).digest('hex').slice(0, 32);
  const slots = n => Array.from({length: n}, () => ({item: '', count: 0}));
  const hotbar = slots(5), bagSlots = slots(20);
  hotbar[0] = {item: 'beer', count: 100};
  hotbar[1] = {item: 'sugar', count: 10};
  bagSlots[0] = {item: 'water', count: 1};
  fs.writeFileSync(path.join(data, 'players.json'), JSON.stringify({version: 2, profiles: {[id]: {
    id, name: 'Fall Explorer', callName: 'Fall Explorer', username: 'fall_explorer', coins: 500,
    inventoryVersion: 3, bagSlots, hotbar, selectedSlot: 0, levels: {},
    homeIsland: {owner: id, visitors: false, builders: {}, chest: {}, objects: []},
  }}}));
  const executable = path.join(output, process.platform === 'win32' ? 'server.exe' : 'server');
  const go = process.env.CK_GO || (process.platform === 'win32' ? 'C:/Program Files/Go/bin/go.exe' : 'go');
  const build = spawnSync(go, ['build', '-o', executable, '.'], {cwd: root, windowsHide: true, encoding: 'utf8'});
  assert.equal(build.status, 0, build.stderr || build.stdout);
  const port = await new Promise(resolve => {
    const probe = net.createServer();
    probe.listen(0, '127.0.0.1', () => {const port = probe.address().port; probe.close(() => resolve(port));});
  });
  const base = 'http://127.0.0.1:' + port;
  let serverLog = '', browser, page, snapshot, activeSocket, walking = false, walked = 0, lastPosition;
  const pageErrors = [], actionErrors = [], failed = [], actions = [];
  const server = spawn(executable, ['-dev', '-addr', '127.0.0.1:' + port, '-data', data], {
    cwd: root, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'],
  });
  server.stdout.on('data', chunk => serverLog += chunk);
  server.stderr.on('data', chunk => serverLog += chunk);
  const source = fs.readFileSync(path.join(root, 'web/state-sync.js'), 'utf8');
  const {createSnapshotDecoder} = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));
  async function until(check, label, timeout = 15000) {
    const started = Date.now();
    while (!await check()) {
      assert.ok(Date.now() - started < timeout, label + ' ' + actionErrors.join('; '));
      await new Promise(resolve => setTimeout(resolve, 40));
    }
  }
  const shot = name => page.screenshot({path: path.join(output, name + '.png')});
  const action = (action, extra = {}) => page.evaluate(message => window.__fallSocket.send(JSON.stringify(message)), {type: 'action', action, ...extra});
  try {
    await until(() => fetch(base + '/api/world').then(r => r.ok).catch(() => false), 'server start');
    browser = await chromium.launch({
      executablePath: process.env.CK_CHROME || (process.platform === 'win32' ? 'C:/Program Files/Google/Chrome/Application/chrome.exe' : undefined),
      headless: true, args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--enable-webgl'],
    });
    const context = await browser.newContext({viewport: {width: 960, height: 720}});
    page = await context.newPage();
    await page.addInitScript(() => {
      const send = WebSocket.prototype.send;
      WebSocket.prototype.send = function(payload) {window.__fallSocket = this; return send.call(this, payload);};
    });
    page.on('pageerror', e => pageErrors.push(e.message));
    page.on('response', r => {if (r.status() >= 400) failed.push(r.status() + ' ' + r.url());});
    page.on('websocket', socket => {
      activeSocket = socket;
      snapshot = null;
      const decoder = createSnapshotDecoder();
      socket.on('framesent', ({payload}) => {const message = JSON.parse(payload.toString()); if (message.type === 'action') actions.push(message);});
      socket.on('framereceived', ({payload}) => {
        if (socket !== activeSocket) return;
        const message = JSON.parse(payload.toString()), next = decoder.apply(message);
        if (next) {
          snapshot = next;
          if (walking && lastPosition) walked += Math.hypot(snapshot.me.x - lastPosition.x, snapshot.me.z - lastPosition.z);
          lastPosition = {x: snapshot.me.x, z: snapshot.me.z};
        }
        if (message.type === 'error') actionErrors.push(message.message);
      });
    });
    await page.goto(base, {waitUntil: 'networkidle'});
    await page.locator('#email').fill(email); await page.locator('#email-submit').click();
    await page.locator('#code-stage').waitFor({state: 'visible'});
    if (!await page.locator('#login-code').inputValue()) await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    await page.locator('#code-submit').click(); await page.locator('#start-button').waitFor({state: 'visible'});
    await until(() => snapshot?.me, 'snapshot');
    await page.locator('#start-button').click(); await page.waitForFunction(() => !!document.pointerLockElement);
    await action('teleport_home'); await until(() => snapshot.me.island === id, 'home arrival');
    assert.equal(snapshot.me.fallChance, .1);
    await page.keyboard.press('q'); await until(() => snapshot.me.alcoholDrinks === 1, 'Q consumes alcoholic drink');
    assert.equal(snapshot.me.fallChance, 1.1);
    let nextDrinkAt = snapshot.time + 1.06;
    await until(() => snapshot.time >= nextDrinkAt, 'drink cooldown');
    await action('drink', {item: 'water'}); await until(() => !snapshot.me.inventory.water, 'water consumed');
    assert.equal(snapshot.me.fallChance, 1.1, 'nonalcoholic drink must not change fall odds');
    nextDrinkAt = snapshot.time + 1.06;
    for (let drinks = 2; drinks <= 100; drinks++) {
      await until(() => snapshot.time >= nextDrinkAt, 'drink cooldown');
      await action('drink', {item: 'beer'}); await until(() => snapshot.me.alcoholDrinks === drinks, 'drink ' + drinks);
      nextDrinkAt = snapshot.time + 1.06;
      if (drinks % 25 === 0) console.log('Consumed ' + drinks + ' drinks through normal server actions.');
    }
    assert.equal(snapshot.me.fallChance, 100); assert.equal(snapshot.me.fallen, false);
    assert.match(await page.locator('#buffs').innerText(), /100\.0% \/ 50m/);
    await shot('standing-before-walk');
    walking = true;
    await page.keyboard.down('w'); await until(() => snapshot.me.z <= -18, 'first forty metres'); await page.keyboard.up('w');
    // Let the stopped endpoint reach the snapshot stream before reversing;
    // otherwise one sampled segment hides the out-and-back turnaround distance.
    await page.waitForTimeout(350);
    assert.equal(snapshot.me.fallen, false, 'less than fifty metres must not roll');
    await page.keyboard.down('s'); await until(() => snapshot.me.fallen, 'guaranteed fall at fifty metres'); await page.keyboard.up('s');
    walking = false;
    assert.ok(walked >= 49.9 && walked <= 50.5, 'fall at measured fifty metres: ' + walked);
    assert.equal(snapshot.me.posture, 'lying'); assert.equal(snapshot.me.grounded, true);
    const fallPosition = {x: snapshot.me.x, z: snapshot.me.z};
    await shot('fallen-recovery');
    await action('teleport_main'); await action('stand');
    await until(() => actionErrors.length === 2, 'early actions rejected');
    assert.match(actionErrors[0], /Stand up before/); assert.match(actionErrors[1], /Rest for/);
    await page.keyboard.down('w'); await page.keyboard.down('Space'); await page.waitForTimeout(400);
    await page.keyboard.up('w'); await page.keyboard.up('Space');
    assert.equal(snapshot.me.fallen, true); assert.equal(snapshot.me.x, fallPosition.x); assert.equal(snapshot.me.z, fallPosition.z);
    assert.equal(snapshot.me.y, 0); assert.equal(snapshot.me.island, id);
    await until(() => snapshot.me.standIn === 0, 'two-second recovery');
    assert.equal(snapshot.me.fallen, true, 'recovery must wait for stand input');
    await shot('ready-to-stand');
    await page.keyboard.press('e'); await until(() => !snapshot.me.fallen, 'E stands');
    assert.equal(snapshot.me.posture, ''); assert.equal(snapshot.me.fallChance, 100);
    await until(() => page.locator('#interaction-prompt').isHidden(), 'stand prompt clears');
    await shot('standing-after-fall');
    await page.reload({waitUntil: 'networkidle'}); await until(() => snapshot?.me.alcoholDrinks === 100, 'reconnect keeps alcohol');
    assert.equal(snapshot.me.fallChance, 100);
    assert.deepEqual(pageErrors, []); assert.deepEqual(failed, []); assert.equal(actionErrors.length, 2);
    console.log('PASS: 0.1% base, +1 point per alcohol, no effect from water, 100% cap, actual 50m walk, lying camera/HUD, two-second action/movement lock, E stand, reconnect retention.');
    console.log('Screenshots: ' + output);
  } catch (error) {
    if (page) await shot('failure').catch(() => {});
    console.error({pageErrors, actionErrors, failed, walked, me: snapshot?.me});
    throw error;
  } finally {
    if (browser) await browser.close();
    server.kill();
    await Promise.race([new Promise(resolve => server.once('exit', resolve)), new Promise(resolve => setTimeout(resolve, 3000))]);
    fs.writeFileSync(path.join(output, 'server.log'), serverLog);
    fs.writeFileSync(path.join(output, 'actions.json'), JSON.stringify(actions, null, 2));
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
