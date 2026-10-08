// Real Chrome controls, authoritative workouts, and a temporary hut spawn only.
const assert = require('node:assert/strict'), fs = require('node:fs'), path = require('node:path');
const crypto = require('node:crypto'), net = require('node:net'), { spawn, spawnSync } = require('node:child_process');
const { chromium } = require(require.resolve('playwright', { paths: [path.resolve(__dirname, '../.tools')] }));

(async () => {
  const root = path.resolve(__dirname, '..'), out = path.join(root, 'test-results', 'cookie-practice-' + Date.now());
  const data = path.join(out, 'data'); fs.mkdirSync(data, { recursive: true });
  const email = 'cookie-practice@example.com', id = 'u_' + crypto.createHash('sha256').update(email).digest('hex').slice(0, 32);
  const slots = n => Array.from({ length: n }, () => ({ item: '', count: 0 }));
  const profile = { id, name: 'Practice Pal', callName: 'Practice Pal', username: 'practice_pal', coins: 100,
    inventoryVersion: 3, hotbar: slots(5), bagSlots: slots(20), levels: {}, discovered: { sugar: true } };
  profile.hotbar[0] = { item: 'sugar', count: 3 };
  fs.writeFileSync(path.join(data, 'players.json'), JSON.stringify({ version: 3, profiles: { [id]: profile } }));
  const world = path.join(root, 'internal/game/world.go'), source = fs.readFileSync(world, 'utf8');
  const changed = source.replace(/(\tw\.spawn\(p\)\r?\n)(\tw\.players\[id\] = p)/, '$1\tp.x, p.y, p.z = 215, 12, 183\n$2');
  assert.notEqual(source, changed, 'isolated hut spawn anchor');
  const replacement = path.join(out, 'world.txt'), overlay = path.join(out, 'overlay.json');
  fs.writeFileSync(replacement, changed); fs.writeFileSync(overlay, JSON.stringify({ Replace: { [world]: replacement } }));
  const exe = path.join(out, process.platform === 'win32' ? 'server.exe' : 'server');
  const go = process.env.CK_GO || (process.platform === 'win32' ? 'C:/Program Files/Go/bin/go.exe' : 'go');
  const built = spawnSync(go, ['build', '-overlay', overlay, '-o', exe, '.'], { cwd: root, windowsHide: true, encoding: 'utf8' });
  assert.equal(built.status, 0, built.stderr || built.stdout);
  const port = await new Promise((resolve, reject) => { const s = net.createServer(); s.once('error', reject); s.listen(0, '127.0.0.1', () => { const p = s.address().port; s.close(() => resolve(p)); }); });
  const base = 'http://127.0.0.1:' + port;
  let browser, page, snapshot, log = '';
  const errors = [], failed = [], actions = [];
  const server = spawn(exe, ['-dev', '-addr', '127.0.0.1:' + port, '-data', data], { cwd: root, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
  server.stdout.on('data', c => log += c); server.stderr.on('data', c => log += c);
  const { createSnapshotDecoder } = await import('data:text/javascript;base64,' + Buffer.from(fs.readFileSync(path.join(root, 'web/state-sync.js'), 'utf8')).toString('base64'));
  async function until(check, label, timeout = 20000) { const start = Date.now(); while (!await check()) { assert.ok(Date.now() - start < timeout, label + '; ' + errors.join('; ')); await new Promise(r => setTimeout(r, 80)); } }
  const shot = name => page.screenshot({ path: path.join(out, name + '.png') });
  const exercise = () => snapshot.cookiePractice?.exercise;
  async function resume() {
    if (await page.locator('#modal').isVisible()) await page.locator('#close-modal').click();
    if (await page.locator('#play-overlay').isVisible()) await page.locator('#play-button').click();
    if (await page.locator('#capture-prompt').isVisible()) await page.locator('#capture-prompt').click();
    await page.waitForFunction(() => !!document.pointerLockElement && window.__practiceDebug().locked);
  }
  async function travel(x, z) {
    await resume(); const start = Date.now();
    try {
      while (Math.hypot(snapshot.me.x - x, snapshot.me.z - z) > .2) {
        assert.ok(Date.now() - start < 22000, 'route blocked toward ' + x + ',' + z + ' at ' + snapshot.me.x + ',' + snapshot.me.z);
        const dx = x - snapshot.me.x, dz = z - snapshot.me.z, d = Math.max(1, Math.hypot(dx, dz));
        await page.evaluate(input => window.__practiceInput = input, { x: dx / d, z: dz / d, sprint: false });
        await page.waitForTimeout(70);
      }
    } finally { await page.evaluate(() => window.__practiceInput = null); }
    await page.waitForTimeout(180);
  }
  async function face(x, z, pitch = -.03) {
    await resume(); const yaw = Math.atan2(-(x - snapshot.me.x), -(z - snapshot.me.z));
    await page.evaluate(({ yaw, pitch }) => {
      const d = window.__practiceDebug(), e = new MouseEvent('mousemove');
      Object.defineProperty(e, 'movementX', { value: (d.yaw - yaw) / .002 });
      Object.defineProperty(e, 'movementY', { value: (d.pitch - pitch) / .002 }); document.dispatchEvent(e);
    }, { yaw, pitch });
    await page.waitForTimeout(240);
  }
  async function mount(station) {
    const s = snapshot.cookiePractice.stations.find(s => s.id === station);
    await face(s.x, s.z, -.2); await page.keyboard.press('e');
    await until(() => exercise().station === station, 'E mounts ' + station);
  }
  async function stop() {
    await page.keyboard.press('q'); await until(() => !exercise().station, 'Q steps off apparatus');
  }
  async function pressSet(station, stat) {
    await mount(station); await until(() => exercise().restIn === 0, 'workout recovery');
    await page.keyboard.down('Space'); await until(() => exercise().progress === 1, 'first real Space repetition');
    await page.waitForTimeout(1500); assert.equal(exercise().progress, 1, 'holding Space never repeats');
    await page.keyboard.up('Space'); await page.waitForTimeout(140);
    for (let rep = 2; rep <= 10; rep++) {
      await page.keyboard.down('Space'); await page.waitForTimeout(100); await page.keyboard.up('Space');
      await until(() => rep === 10 ? snapshot.me.levels[stat] === 1 : exercise().progress === rep, stat + ' repetition ' + rep);
      if (rep === 5) await shot(station + '-controlled-repetitions');
      await page.waitForTimeout(780);
    }
    assert.equal(snapshot.me.levels[stat], 1); await stop();
  }
  try {
    await until(() => fetch(base + '/healthz').then(r => r.ok).catch(() => false), 'server start');
    browser = await chromium.launch({ executablePath: process.env.CK_CHROME || (process.platform === 'win32' ? 'C:/Program Files/Google/Chrome/Application/chrome.exe' : undefined), headless: true, args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--enable-webgl'] });
    page = await browser.newPage({ viewport: { width: 1440, height: 1000 } });
    await page.addInitScript(() => { const send = WebSocket.prototype.send; WebSocket.prototype.send = function (payload) { try { const m = JSON.parse(payload); if (m.type === 'input' && window.__practiceInput) payload = JSON.stringify({ ...m, ...window.__practiceInput }); } catch {} return send.call(this, payload); }; });
    await page.route('**/game.js', async route => {
      const response = await route.fetch(), source = await response.text();
      await route.fulfill({ response, body: source.replace('function receiveSnapshot(snapshot){', 'window.__practiceDebug=()=>({locked:state.locked,yaw:state.yaw,pitch:state.pitch});function receiveSnapshot(snapshot){').replace('renderer.render(scene,camera);', 'window.__practiceFrames=(window.__practiceFrames||0)+1;if(window.__practiceFrames%3===0)renderer.render(scene,camera);') });
    });
    page.on('pageerror', e => errors.push(e.message)); page.on('response', r => { if (r.status() >= 400) failed.push(r.status() + ' ' + r.url()); });
    page.on('websocket', socket => { const decoder = createSnapshotDecoder(); socket.on('framereceived', ({ payload }) => { try { const m = JSON.parse(payload.toString()), next = decoder.apply(m); if (next) snapshot = next; if (m.type === 'error') errors.push(m.message); } catch (e) { errors.push(e.message); } }); socket.on('framesent', ({ payload }) => { const m = JSON.parse(payload.toString()); if (m.type === 'action') actions.push(m); }); });
    await page.goto(base, { waitUntil: 'networkidle' }); await page.locator('#email').fill(email); await page.locator('#email-submit').click(); await page.locator('#code-stage').waitFor({ state: 'visible' });
    if (!await page.locator('#login-code').inputValue()) await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    await page.locator('#code-submit').click(); await page.locator('#start-button').waitFor({ state: 'visible' }); await until(() => snapshot?.me, 'login'); await page.locator('#start-button').click(); await resume();
    assert.equal(snapshot.me.y, 12); await face(215, 181.5); await page.keyboard.press('e');
    await page.getByRole('button', { name: /^Eat silver cookie/ }).click(); await until(() => snapshot.me.island === 'cookie_practice', 'silver cookie enters peaceful practice'); await resume();
    assert.equal(snapshot.cookiePractice.fee, 2); assert.equal(snapshot.cookiePractice.levelCap, 10); assert.equal(snapshot.me.inventory.sugar, 3);
    console.log('Entered real silver-cookie practice center.');
    await travel(-4, 18); await face(-10, 5, -.06); await shot('practice-center-apparatus-and-range');
    await travel(-4, -11); await travel(-16, -11); await face(-16, -21, .08);
    await page.mouse.down(); await page.mouse.up(); await until(() => snapshot.cookiePractice.hits === 1, 'real supplied cookie hits stationary target');
    assert.equal(snapshot.me.inventory.sugar, 3); const movingX = snapshot.cookiePractice.targets.find(t => t.moving).x;
    await page.waitForTimeout(800); assert.notEqual(snapshot.cookiePractice.targets.find(t => t.moving).x, movingX);
    await shot('stationary-hit-and-moving-target');
    await travel(12, -11);
    const predictedX = 12 + Math.sin((snapshot.time + .68) * .8) * 5;
    await face(predictedX, -21, .08); await page.mouse.down(); await page.mouse.up();
    await until(() => snapshot.cookiePractice.hits === 2, 'real supplied cookie hits moving target');
    await shot('moving-target-hit');
    await travel(-12, -11); await travel(-12, 9); await travel(-16, 8.7); await mount('practice_treadmill');
    await page.waitForTimeout(1500); assert.equal(exercise().progress, 0, 'idle apparatus grants no workout'); assert.equal(snapshot.me.coins, 100);
    await page.keyboard.down('w'); await until(() => exercise().progress >= 4, 'holding W drives real treadmill work'); await shot('running-treadmill-work');
    await until(() => snapshot.me.levels.stamina === 1, 'twenty real treadmill seconds complete a set', 30000); await page.keyboard.up('w');
    assert.equal(snapshot.me.coins, 98); assert.ok(Math.abs(snapshot.me.x + 16) < .01); await stop(); console.log('Treadmill awards only completed work.');
    await travel(-12, 9); await travel(-8, 8.3); await pressSet('practice_bench_press', 'arms'); assert.equal(snapshot.me.coins, 96);
    await travel(-8, 17); await travel(-16, 16.3); await pressSet('practice_leg_press', 'legs'); assert.equal(snapshot.me.coins, 94); console.log('Bench and leg press require controlled Space repetitions.');
    await travel(-8, 18); await travel(10, 18); await face(10, -5, -.04); await shot('physical-swimming-pool'); await travel(10, 13);
    await until(() => exercise().station === 'practice_pool', 'enter swimming lane'); assert.equal(snapshot.me.y, -.6);
    await travel(10, -5); assert.equal(snapshot.me.levels.swim || 0, 0, 'one length is not a full set'); assert.equal(snapshot.me.coins, 94);
    await face(10, 13, -.07); await shot('pool-far-end-and-return-marker'); await travel(10, 13); await until(() => snapshot.me.levels.swim === 1, 'actual round trip earns swimming skill');
    assert.equal(snapshot.me.coins, 92); assert.equal(snapshot.me.inventory.sugar, 3); assert.equal(snapshot.cookiePractice.hits, 2);
    await travel(10, 18); await travel(0, 20); await face(0, 22); await shot('completed-workout-return-cookie'); await page.keyboard.press('e');
    await until(() => snapshot.me.island === '', 'return cookie restores tree hut'); assert.equal(snapshot.me.y, 12); assert.equal(snapshot.me.inventory.sugar, 3);
    assert.deepEqual(errors, [], 'browser and server errors'); assert.deepEqual(failed, [], 'HTTP errors');
    console.log('PASS: real silver-cookie entry, stationary and moving targets, treadmill, bench press, leg press, actual swimming lanes, exactly eight coins for four completed sets, unchanged cookie inventory, and return to hut.');
    console.log('Screenshots: ' + out);
  } catch (error) {
    if (page) await shot('failure').catch(() => {}); console.error({ errors, failed, me: snapshot?.me, practice: snapshot?.cookiePractice, actions: actions.slice(-15) }); throw error;
  } finally {
    await browser?.close(); server.kill(); if (server.exitCode === null) await Promise.race([new Promise(r => server.once('exit', r)), new Promise(r => setTimeout(r, 3000))]);
    fs.writeFileSync(path.join(out, 'server.log'), log); fs.writeFileSync(path.join(out, 'actions.json'), JSON.stringify(actions, null, 2));
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
