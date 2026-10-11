// Isolated Chrome/Go regression: replay real animal snapshots beside the swimmer.
// CK_REPRO_ORIGINAL=1 also serves the previous unsafe dispatch to prove the freeze.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const net = require('node:net');
const {spawn, spawnSync} = require('node:child_process');
const {chromium} = require(require.resolve('playwright', {paths: [path.resolve(__dirname, '../.tools')]}));

(async () => {
  const root = path.resolve(__dirname, '..');
  const output = path.join(root, 'test-results', 'animal-browser-' + Date.now());
  const data = path.join(output, 'data');
  fs.mkdirSync(data, {recursive: true});
  const executable = path.join(output, process.platform === 'win32' ? 'server.exe' : 'server');
  const go = process.env.CK_GO || (process.platform === 'win32' ? 'C:/Program Files/Go/bin/go.exe' : 'go');
  const build = spawnSync(go, ['build', '-o', executable, '.'], {cwd: root, windowsHide: true, encoding: 'utf8'});
  assert.equal(build.status, 0, build.stderr || build.stdout);
  const port = await new Promise(resolve => {
    const probe = net.createServer();
    probe.listen(0, '127.0.0.1', () => {const port = probe.address().port; probe.close(() => resolve(port));});
  });
  const base = 'http://127.0.0.1:' + port;
  let browser, page, serverLog = '', original = process.env.CK_REPRO_ORIGINAL === '1';
  const errors = [], actions = [];
  const server = spawn(executable, ['-dev', '-addr', '127.0.0.1:' + port, '-data', data], {cwd: root, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe']});
  server.stdout.on('data', chunk => serverLog += chunk);
  server.stderr.on('data', chunk => serverLog += chunk);
  async function until(check, label, timeout = 20000) {
    const started = Date.now();
    while (!await check()) {
      assert.ok(Date.now() - started < timeout, label + ': ' + errors.join('; '));
      await new Promise(resolve => setTimeout(resolve, 50));
    }
  }
  const read = () => page.evaluate(() => window.__animalSmoke.read());
  const replay = options => page.evaluate(options => window.__animalSmoke.replay(options), options);
  try {
    await until(() => fetch(base + '/healthz').then(r => r.ok).catch(() => false), 'server start');
    browser = await chromium.launch({executablePath: process.env.CK_CHROME || (process.platform === 'win32' ? 'C:/Program Files/Google/Chrome/Application/chrome.exe' : undefined), headless: true, args: ['--use-angle=swiftshader', '--enable-unsafe-swiftshader', '--enable-webgl']});
    page = await browser.newPage({viewport: {width: 960, height: 720}});
    await page.route('**/game.js', async route => {
      const response = await route.fetch();
      let source = await response.text();
      if (original) source = source.replaceAll('n.kind?.startsWith(', 'n.kind.startsWith(');
      const hook = `
        let animalReplaySource, animalReplayBlocked=false;
        window.__animalSmoke={
          read:()=>({ready:!!state.snapshot,locked:state.locked,frames:renderer.info.render.frame,compass:$('compass-direction').textContent,position:$('position-label').textContent,label:$('interaction-label').textContent,modal:state.modal,nearest:state.nearest,camera:camera.position.toArray(),planned:[...state.animals.values()].filter(o=>o.userData.animal.motion?.kind==='orbit').slice(0,1).map(o=>({id:o.userData.animal.id,motion:o.userData.animal.motion,x:o.position.x,z:o.position.z,displayX:o.userData.animal.x,displayZ:o.userData.animal.z})),source:animalReplaySource?.animals?.map(a=>({id:a.id,x:a.x,z:a.z}))}),
          path:()=>{
            animalReplaySource=structuredClone(state.snapshot);animalReplayBlocked=true;
            const animal=animalReplaySource.animals.find(a=>a.species==='fish'&&a.motion?.kind==='orbit'&&!a.egg);
            if(!animal)throw Error('Missing planned fish route');
            animalReplaySource.animals=[animal];animalReplaySource.nodes=[];
            Object.assign(animalReplaySource.me,{x:animal.x+1,y:-.6,z:animal.z,island:'',inCave:false});
            return animal.id;
          },
          advance:()=>{
            receiveSnapshot({...animalReplaySource,time:state.snapshot.time+.1,seq:(state.snapshot.seq||1)+1,animalReplay:true});
          },
          replay:({species='fish',changes={},offset=1,yaw=0}={})=>{
            animalReplaySource??=structuredClone(state.snapshot);
            animalReplayBlocked=true;
            const snapshot=structuredClone(animalReplaySource), animal=snapshot.animals.find(a=>a.species===species&&!a.egg);
            if(!animal)throw Error('Missing server animal '+species);
            Object.assign(animal,{need:'',egg:false,motion:null},changes);
            snapshot.animals=[animal];snapshot.nodes=[];
            Object.assign(snapshot.me,{x:animal.x+offset,y:-.6,z:animal.z,island:'',inCave:false});
            snapshot.time=state.snapshot.time+.2;snapshot.seq=(state.snapshot.seq||1)+1;snapshot.animalReplay=true;
            state.yaw=yaw;receiveSnapshot(snapshot);
            return animal.id;
          },
          close:()=>closeModal({resume:false})
        };
        function receiveSnapshot(snapshot){
          if(animalReplayBlocked&&!snapshot.animalReplay)return;
      `;
      source = source.replace('function receiveSnapshot(snapshot){', hook);
      await route.fulfill({response, body: source});
    });
    page.on('pageerror', error => {if (errors.length < 30) errors.push(error.message);});
    page.on('websocket', socket => socket.on('framesent', ({payload}) => {
      const message = JSON.parse(payload.toString());
      if (message.type === 'action') actions.push(message);
    }));
    await page.goto(base, {waitUntil: 'networkidle'});
    await page.locator('#email').fill('animal-smoke@example.com');
    await page.locator('#email-submit').click();
    await page.locator('#code-stage').waitFor({state: 'visible'});
    if (!await page.locator('#login-code').inputValue()) await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    await page.locator('#code-submit').click();
    await page.locator('#start-button').waitFor({state: 'visible'});
    await until(async () => (await read()).ready, 'snapshot');
    await page.locator('#start-button').click();
    if (original) {
      await replay({species: 'fish'});
      await until(() => errors.some(e => e.includes('startsWith')), 'original fish exception');
      const frozen = await read();
      await replay({species: 'fish', offset: 2.5, yaw: Math.PI});
      await until(async () => (await read()).compass !== frozen.compass, 'HUD updates despite freeze');
      await page.waitForTimeout(350);
      const after = await read();
      assert.equal(after.frames, frozen.frames, 'original fish proximity skips every 3D render');
      assert.notEqual(after.position, frozen.position, 'map position still updates');
      await page.screenshot({path: path.join(output, 'original-freeze.png')});
      console.log('REPRODUCED: nearby fish freezes every 3D frame while direction and location keep updating.');
      original = false;
      await page.reload({waitUntil: 'networkidle'});
      await page.locator('#start-button').waitFor({state: 'visible'});
      await until(async () => (await read()).ready, 'fresh snapshot');
      await page.locator('#start-button').click();
      errors.length = 0;
    }
    await page.evaluate(() => window.__animalSmoke.path());
    await page.evaluate(() => window.__animalSmoke.advance());
    const routeBefore=await read();
    for(let frame=0;frame<12;frame++){
      await page.waitForTimeout(100);
      await page.evaluate(() => window.__animalSmoke.advance());
    }
    await until(async()=>{const s=await read();return s.planned[0]?.displayX===s.planned[0]?.x&&s.planned[0]?.displayZ===s.planned[0]?.z;},'route frame uses the projected display coordinates');
    const routeAfter=await read();
    assert.ok(routeAfter.frames>routeBefore.frames+5,'planned animals continue rendering');
    assert.deepEqual(routeAfter.planned[0].motion,routeBefore.planned[0].motion,'time-only updates retain the original route');
    assert.ok(Math.hypot(routeAfter.planned[0].x-routeBefore.planned[0].x,routeAfter.planned[0].z-routeBefore.planned[0].z)>.01,'client projects fish movement from the unchanged route');
    assert.equal(routeAfter.planned[0].displayX,routeAfter.planned[0].x,'nearby/UI coordinates match rendered fish');
    assert.equal(routeAfter.planned[0].displayZ,routeAfter.planned[0].z);
    assert.deepEqual(routeAfter.source,routeBefore.source,'animation leaves the source snapshot unchanged');
    await page.waitForTimeout(400);
    const stalled=await read();
    await page.waitForTimeout(400);
    const still=await read();
    assert.equal(still.planned[0].x,stalled.planned[0].x,'stalled snapshots hold the last confirmed route position');
    assert.equal(still.planned[0].z,stalled.planned[0].z);
    // Restore the complete animal set for the existing interaction regressions.
    await page.reload({waitUntil:'networkidle'});
    await page.locator('#start-button').waitFor({state:'visible'});
    await until(async ()=>(await read()).ready,'fresh snapshot after route regression');
    await page.locator('#start-button').click();
    for (const scenario of [
      {species: 'fish', label: 'Fish'},
      {species: 'fish', changes: {need: 'wounded'}, label: 'Help the wounded fish', modal: 'animal'},
      {species: 'turtle', changes: {need: 'wounded'}, label: 'Help the wounded turtle', modal: 'animal'},
      {species: 'turtle', changes: {need: 'trapped'}, label: 'Free the turtle'},
      {species: 'fish', changes: {egg: true, hatchIn: 43}, label: 'A fish egg', egg: true},
    ]) {
      await replay(scenario);
      await until(async () => (await read()).label === scenario.label, scenario.label);
      const before = await read();
      await replay({...scenario, offset: 2.5, yaw: Math.PI / 2});
      await until(async () => {const now = await read(); return now.frames > before.frames + 1 && now.compass === 'W' && now.position !== before.position;}, 'render, compass and location advance near ' + scenario.label);
      if (!await page.evaluate(() => !!document.pointerLockElement)) await page.locator('#world').click();
      await until(async () => (await read()).locked, 'game receives pointer lock');
      const actionCount = actions.length;
      await page.keyboard.press('e');
      if (scenario.modal) {
        await until(async () => (await read()).modal === scenario.modal, 'animal rescue menu');
        await page.evaluate(() => window.__animalSmoke.close());
      } else if (!scenario.egg) {
        await until(() => actions.length > actionCount, 'animal interact action');
        assert.equal(actions.at(-1).action, 'interact');
      } else {
        await page.waitForTimeout(150);
        assert.equal(actions.length, actionCount, 'egg help does not disturb it');
      }
      assert.deepEqual(errors, []);
    }
    await page.screenshot({path: path.join(output, 'fixed-fish-egg.png')});
    console.log('PASS: Chrome projects unchanged fish routes from time-only updates, holds on stalled streams, preserves source snapshots, renders continuously beside healthy/wounded fish, wounded/trapped turtles and eggs; E dispatch works; no browser errors.');
    console.log('Artifacts: ' + output);
  } catch (error) {
    if (page) console.error('Browser state:', await read().catch(() => null));
    if (page) await page.screenshot({path: path.join(output, 'failure.png')}).catch(() => {});
    throw error;
  } finally {
    fs.writeFileSync(path.join(output, 'server.log'), serverLog);
    await browser?.close();
    server.kill();
    await new Promise(resolve => {if (server.exitCode !== null) return resolve(); server.once('exit', resolve); setTimeout(resolve, 3000);});
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
