// Real Chrome and an isolated save. Only the initial spawn/clock use a Go overlay;
// boarding, movement, camel ownership and completed rides use production logic.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const net = require('node:net');
const {spawn, spawnSync} = require('node:child_process');
const {chromium} = require(require.resolve('playwright', {paths:[path.resolve(__dirname, '../.tools')]}));

(async () => {
  const root = path.resolve(__dirname, '..');
  const out = path.join(root, 'test-results', 'roller-coaster-' + Date.now());
  const data = path.join(out, 'data');
  fs.mkdirSync(data, {recursive:true});
  const email = 'roller-coaster@example.com';
  const id = 'u_' + crypto.createHash('sha256').update(email).digest('hex').slice(0, 32);
  const camelID = id + '_camel_1';
  const slots = n => Array.from({length:n}, () => ({item:'', count:0}));
  const station = {x:165, z:-161};
  const spawnAt = {x:station.x, z:station.z + 10};
  const camelAt = {x:station.x + 7, z:station.z + 6};
  const camel = {id:camelID, name:'Cinnamon', x:camelAt.x, y:0, z:camelAt.z, yaw:0,
    island:'', inCave:false, rentalUntil:0, traitsVersion:1,
    speed:12.5, jump:7.5, stamina:145, happiness:100};
  const profile = {id, name:'Coaster Explorer', callName:'Coaster Explorer', username:'coaster_explorer',
    coins:700, inventoryVersion:3, hotbar:slots(5), bagSlots:slots(20), selectedSlot:0,
    levels:{}, camels:[camel], discovered:{sugar:true}};
  profile.hotbar[0] = {item:'sugar', count:8};
  profile.bagSlots[0] = {item:'wood', count:23};
  profile.bagSlots[1] = {item:'berry', count:7};
  fs.writeFileSync(path.join(data, 'players.json'), JSON.stringify({version:3, profiles:{[id]:profile}}));
  const worldFile = path.join(root, 'internal/game/world.go');
  const worldSource = fs.readFileSync(worldFile, 'utf8');
  const spawnAnchor = /(\tw\.spawn\(p\)\r?\n)(\tw\.players\[id\] = p)/;
  assert.match(worldSource, spawnAnchor, 'isolated coaster spawn anchor');
  const changed = worldSource.replace('\tw.seedMap()', '\tw.time = 60\n\tw.seedMap()')
    .replace(spawnAnchor, '$1\tp.x, p.z, p.yaw = ' + spawnAt.x + ', ' + spawnAt.z + ', 0\n$2');
  const overlaySource = path.join(out, 'world.go.txt');
  const overlay = path.join(out, 'overlay.json');
  fs.writeFileSync(overlaySource, changed);
  fs.writeFileSync(overlay, JSON.stringify({Replace:{[worldFile]:overlaySource}}));
  const exe = path.join(out, process.platform === 'win32' ? 'server.exe' : 'server');
  const go = process.env.CK_GO || (process.platform === 'win32' ? 'C:/Program Files/Go/bin/go.exe' : 'go');
  const built = spawnSync(go, ['build', '-overlay', overlay, '-o', exe, '.'], {cwd:root, windowsHide:true, encoding:'utf8'});
  assert.equal(built.status, 0, built.stderr || built.stdout);
  const port = await new Promise((resolve, reject) => {
    const probe = net.createServer(); probe.once('error', reject);
    probe.listen(0, '127.0.0.1', () => {const port = probe.address().port; probe.close(() => resolve(port));});
  });
  const base = 'http://127.0.0.1:' + port;
  let browser, page, snapshot, log = '';
  const errors = [], failed = [], actions = [], history = [];
  const server = spawn(exe, ['-dev', '-addr', '127.0.0.1:' + port, '-data', data],
    {cwd:root, windowsHide:true, stdio:['ignore','pipe','pipe']});
  server.stdout.on('data', chunk => log += chunk); server.stderr.on('data', chunk => log += chunk);
  const decoderURL = 'data:text/javascript;base64,' + Buffer.from(fs.readFileSync(path.join(root, 'web/state-sync.js'), 'utf8')).toString('base64');
  const {createSnapshotDecoder} = await import(decoderURL);
  async function until(check, label, timeout = 20000) {
    const started = Date.now();
    while (!await check()) {
      assert.ok(Date.now() - started < timeout, label + '; ' + errors.join('; '));
      await new Promise(resolve => setTimeout(resolve, 80));
    }
  }
  const shot = name => page.screenshot({path:path.join(out, name + '.png')});
  async function resume() {
    if (await page.locator('#modal').isVisible()) await page.locator('#close-modal').click();
    if (await page.locator('#play-overlay').isVisible()) await page.locator('#play-button').click();
    if (await page.locator('#capture-prompt').isVisible()) await page.locator('#capture-prompt').click();
    await page.waitForFunction(() => !!document.pointerLockElement && window.__coasterDebug().locked);
  }
  async function face(x, z, pitch = -.08) {
    await resume(); const yaw = Math.atan2(-(x - snapshot.me.x), -(z - snapshot.me.z));
    await page.evaluate(({yaw,pitch}) => {
      const view = window.__coasterDebug(), event = new MouseEvent('mousemove');
      Object.defineProperty(event, 'movementX', {value:(view.yaw - yaw) / .002});
      Object.defineProperty(event, 'movementY', {value:(view.pitch - pitch) / .002});
      document.dispatchEvent(event);
    }, {yaw,pitch});
    await page.waitForTimeout(200);
  }
  async function travel(x, z) {
    await resume(); const started = Date.now();
    try {
      while (Math.hypot(snapshot.me.x - x, snapshot.me.z - z) > .3) {
        assert.ok(Date.now() - started < 18000, 'walking blocked at ' + snapshot.me.x + ',' + snapshot.me.z + ' toward ' + x + ',' + z);
        const dx = x - snapshot.me.x, dz = z - snapshot.me.z, length = Math.max(1, Math.hypot(dx,dz));
        await page.evaluate(input => window.__coasterInput = input, {x:dx/length,z:dz/length,sprint:false});
        await page.waitForTimeout(80);
      }
    } finally {await page.evaluate(() => window.__coasterInput = {x:0,z:0,sprint:false});}
    await page.waitForTimeout(160);
  }
  const currentCamel = () => snapshot.me.camels.find(c => c.id === camelID);
  const stableCamel = camel => Object.fromEntries(['id','name','rentalUntil','speed','jump','stamina','happiness','traitsVersion','homePenId','homePenSlot'].map(key => [key,camel[key]]));
  async function board() {
    await face(station.x,station.z);
    await page.waitForFunction(() => /coaster/i.test(document.querySelector('#interaction-label').textContent));
    await page.keyboard.press('e');
    await page.locator('#modal').waitFor({state:'visible'});
    const label=snapshot.me.ridingCamel?'Board with camel':'Board on foot';
    await page.getByRole('button',{name:label,exact:true}).click();
    await until(() => snapshot.rollerCoaster?.ride, 'boarding menu starts the coaster');
    await resume();
    await page.evaluate(() => window.__coasterInput = null);
  }
  async function fullRide(label, withCamel) {
    const origin = {x:snapshot.me.x,y:snapshot.me.y,z:snapshot.me.z};
    const inventory = structuredClone(snapshot.me.inventory), coins = snapshot.me.coins;
    const traits = stableCamel(currentCamel()), first = history.length;
    await board();
    assert.equal(!!snapshot.rollerCoaster.ride.camelId,withCamel);
    assert.equal(snapshot.me.ridingCamel,withCamel);
    if (withCamel) assert.equal(snapshot.me.mountedCamelId,camelID);
    await shot(label + '-departing');
    await until(() => snapshot.rollerCoaster?.ride?.progress > .12, label + ' moves along the track',20000);
    assert.ok(Math.hypot(snapshot.me.x-origin.x,snapshot.me.z-origin.z)>8,'ride has travelled beyond the station');
    assert.match(await page.locator('#roller-coaster-hud').innerText(),/%.*TO STATION/);
    await shot(label + '-moving');
    await until(() => snapshot.rollerCoaster?.ride && snapshot.me.y > 18, label + ' reaches the elevated track',35000);
    const view = await page.evaluate(() => window.__coasterDebug());
    assert.ok([view.camera.x,view.camera.y,view.camera.z].every(Number.isFinite),'camera remains finite');
    assert.ok(view.camera.y>12,'first-person camera follows the elevated cart');
    assert.equal(view.riddenCamelVisible,withCamel,'owned camel visibility follows rider state');
    await shot(label + '-high-track');
    console.log(label + ' reached ' + snapshot.me.y.toFixed(1) + 'm; waiting for the full loop return.');
    await until(() => !snapshot.rollerCoaster?.ride,label + ' automatically returns at the station',50000);
    const samples = history.slice(first).filter(s=>s.coaster?.ride);
    assert.ok(samples.length>30,'ride streamed real moving snapshots');
    assert.ok(Math.max(...samples.map(s=>s.y))>20,'full ride visits the high track');
    assert.ok(Math.max(...samples.map(s=>s.x))-Math.min(...samples.map(s=>s.x))>50,'ride covers the broad loop');
    assert.deepEqual({x:snapshot.me.x,y:snapshot.me.y,z:snapshot.me.z},origin,'automatic exit restores the boarding pose');
    assert.deepEqual(snapshot.me.inventory,inventory,'ride does not consume held or stored items');
    assert.equal(snapshot.me.coins,coins,'the ride is free');
    assert.deepEqual(stableCamel(currentCamel()),traits,'ride preserves owned camel identity and traits');
    assert.equal(snapshot.me.ridingCamel,withCamel,'return preserves the original mounting state');
    if (withCamel) assert.equal(snapshot.me.mountedCamelId,camelID);
    await shot(label + '-returned');
  }
  try {
    await until(() => {if (server.exitCode !== null) throw Error(log); return fetch(base + '/healthz').then(r => r.ok).catch(() => false);}, 'server start');
    browser = await chromium.launch({headless:true,
      executablePath:process.env.CK_CHROME || (process.platform === 'win32' ? 'C:/Program Files/Google/Chrome/Application/chrome.exe' : undefined),
      args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    page = await browser.newPage({viewport:{width:1440,height:1000}});
    await page.addInitScript(() => {
      const send = WebSocket.prototype.send;
      WebSocket.prototype.send = function(payload) {
        const message = JSON.parse(payload);
        if (message.type === 'input' && window.__coasterInput) payload = JSON.stringify({...message,...window.__coasterInput});
        return send.call(this,payload);
      };
    });
    await page.route('**/game.js', async route => {
      const response = await route.fetch(), source = await response.text(), anchor = 'function receiveSnapshot(snapshot){';
      assert.ok(source.includes(anchor), 'read-only coaster debug anchor');
      const debug = 'window.__coasterDebug=()=>({yaw:state.yaw,pitch:state.pitch,locked:state.locked,camera:{x:camera.position.x,y:camera.position.y,z:camera.position.z},riddenCamelVisible:riddenCamel.visible,objects:scene.children.map(o=>({name:o.name,visible:o.visible}))});';
      await route.fulfill({response,body:source.replace(anchor,debug+anchor)});
    });
    page.on('pageerror', error => errors.push(error.message));
    page.on('response', response => {if (response.status() >= 400) failed.push(response.status() + ' ' + response.url());});
    page.on('websocket', socket => {
      const decoder = createSnapshotDecoder();
      socket.on('framesent', ({payload}) => {try {const message = JSON.parse(payload.toString()); if (message.type === 'action') actions.push(message);} catch {}});
      socket.on('framereceived', ({payload}) => {
        try {
          const message = JSON.parse(payload.toString()), next = decoder.apply(message);
          if (next) {
            snapshot = next;
            history.push({time:next.time,x:next.me.x,y:next.me.y,z:next.me.z,posture:next.me.posture,ridingCamel:next.me.ridingCamel,coaster:next.rollerCoaster});
          }
          if (message.type === 'error') errors.push(message.message);
        } catch (error) {errors.push(error.message);}
      });
    });
    await page.goto(base, {waitUntil:'networkidle'});
    await page.locator('#email').fill(email); await page.locator('#email-submit').click();
    await page.locator('#code-stage').waitFor({state:'visible'});
    if (!await page.locator('#login-code').inputValue()) await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    await page.locator('#code-submit').click(); await page.locator('#start-button').waitFor({state:'visible'});
    await until(() => snapshot?.me, 'logged in'); await page.locator('#start-button').click(); await resume();

    assert.ok(snapshot.rollerCoaster,'coaster snapshot is present');
    assert.equal(snapshot.rollerCoaster.duration,48);
    assert.ok(snapshot.rollerCoaster.track.length>32,'authoritative sampled track is available');
    assert.equal(snapshot.me.ridingCamel,false);
    await face(station.x,station.z);await shot('station-approach');
    await travel(station.x,station.z+3);
    await face(station.x,station.z);await shot('station-board-prompt');
    await fullRide('on-foot',false);

    let owned = currentCamel();
    await travel(owned.x-2.2,owned.z);await face(owned.x,owned.z,-.15);
    await page.waitForFunction(() => /camel|Cinnamon/i.test(document.querySelector('#interaction-label').textContent));
    await page.keyboard.press('e');await until(()=>snapshot.me.ridingCamel&&snapshot.me.mountedCamelId===camelID,'mount owned Cinnamon');
    await travel(station.x,station.z+3);
    await shot('mounted-at-station');
    await fullRide('with-camel',true);

    const earlyOrigin={x:snapshot.me.x,y:snapshot.me.y,z:snapshot.me.z};
    const earlyInventory=structuredClone(snapshot.me.inventory),earlyCamel=stableCamel(currentCamel());
    await board();await until(()=>snapshot.rollerCoaster?.ride?.progress>.20,'third ride leaves the station',18000);
    await page.keyboard.press('r');await page.locator('#modal').waitFor({state:'visible'});
    assert.match(await page.locator('#modal-content').innerText(),/Cinnamon.*riding together/);
    await shot('early-return-menu');
    await page.getByRole('button',{name:'Return to station now',exact:true}).click();
    await until(()=>!snapshot.rollerCoaster?.ride,'early return completes');
    assert.deepEqual({x:snapshot.me.x,y:snapshot.me.y,z:snapshot.me.z},earlyOrigin);
    assert.equal(snapshot.me.ridingCamel,true);assert.equal(snapshot.me.mountedCamelId,camelID);
    assert.deepEqual(snapshot.me.inventory,earlyInventory);assert.deepEqual(stableCamel(currentCamel()),earlyCamel);
    await resume();await shot('early-returned-with-camel');

    assert.deepEqual(errors, []); assert.deepEqual(failed, []);
    assert.ok(actions.some(a=>a.action==='coaster_board'||a.action==='interact'),'boarding used a real user action');
    const playerFile=path.join(data,'players',id+'.json');
    await until(()=>{try{return JSON.parse(fs.readFileSync(playerFile,'utf8')).profile.camels.some(c=>c.id===camelID);}catch{return false;}},'owned camel persisted');
    const saved=JSON.parse(fs.readFileSync(playerFile,'utf8')).profile;
    assert.equal(saved.coins,700);assert.deepEqual(saved.inventory,snapshot.me.inventory);
    assert.deepEqual(stableCamel(saved.camels.find(c=>c.id===camelID)),stableCamel(currentCamel()));
    assert.equal(snapshot.me.camels.length,1,'ride never duplicates or replaces the owned camel');
    assert.ok(actions.some(a=>a.action==='coaster_leave'),'early return used the real menu action');
    console.log('PASS: real foot and owned-camel coaster loops, elevated first-person view, automatic and early station returns, unchanged inventory/coins and persisted camel ownership/traits.');
    console.log('Screenshots: ' + out);
  } catch (error) {
    if (page) await page.screenshot({path:path.join(out, 'failure.png')}).catch(() => {});
    fs.writeFileSync(path.join(out, 'failure-snapshot.json'), JSON.stringify({snapshot,errors}, null, 2));
    throw error;
  } finally {
    if (browser) await browser.close();
    server.kill(); await Promise.race([new Promise(resolve => server.once('exit',resolve)),new Promise(resolve => setTimeout(resolve,3000))]);
    fs.writeFileSync(path.join(out, 'server.log'),log);
    fs.writeFileSync(path.join(out, 'actions.json'),JSON.stringify(actions,null,2));
    fs.writeFileSync(path.join(out, 'ride-history.json'),JSON.stringify(history,null,2));
  }
})().catch(error => {console.error(error);process.exitCode = 1;});
