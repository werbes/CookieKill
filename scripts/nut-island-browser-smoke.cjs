// Real Chrome and an isolated save. Only the initial cave spawn uses an overlay;
// island purchases, animal inventory, cage care and reconnect use production code.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const net = require('node:net');
const {spawn, spawnSync} = require('node:child_process');
const {chromium} = require(require.resolve('playwright', {paths:[path.resolve(__dirname, '../.tools')]}));

(async () => {
  const root = path.resolve(__dirname, '..');
  const out = path.join(root, 'test-results', 'nut-island-' + Date.now());
  const data = path.join(out, 'data');
  fs.mkdirSync(data, {recursive:true});
  const email = 'nut-island@example.com';
  const id = 'u_' + crypto.createHash('sha256').update(email).digest('hex').slice(0, 32);
  const slots=n=>Array.from({length:n},()=>({item:'',count:0}));
  const spawnAt={x:120,z:-122};
  const profile={id,name:'Nut Gardener',callName:'Nut Gardener',username:'nut_gardener',coins:1300,inventoryVersion:3,hotbar:slots(5),bagSlots:slots(20),selectedSlot:0,levels:{},discovered:{sugar:true}};
  profile.hotbar[0]={item:'sugar',count:4};profile.bagSlots[0]={item:'pet_squirrel',count:1};profile.bagSlots[1]={item:'pet_cactus_mouse',count:1};profile.bagSlots[2]={item:'wood',count:17};
  fs.writeFileSync(path.join(data, 'players.json'), JSON.stringify({version:3, profiles:{[id]:profile}}));
  const worldFile = path.join(root, 'internal/game/world.go');
  const worldSource = fs.readFileSync(worldFile, 'utf8');
  const spawnAnchor = /(\tw\.spawn\(p\)\r?\n)(\tw\.players\[id\] = p)/;
  assert.match(worldSource, spawnAnchor, 'isolated cave spawn anchor');
  const changed = worldSource.replace('\tw.seedMap()', '\tw.time = 60\n\tw.seedMap()')
    .replace(spawnAnchor, '$1\tp.x, p.z, p.yaw = ' + spawnAt.x + ', ' + spawnAt.z + ', 0\n\tp.y, p.inCave = -24, true\n$2');
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
    await page.waitForFunction(() => !!document.pointerLockElement && window.__nutDebug().locked);
  }
  async function face(x, z, pitch = -.08) {
    await resume(); const yaw = Math.atan2(-(x - snapshot.me.x), -(z - snapshot.me.z));
    await page.evaluate(({yaw,pitch}) => {
      const view = window.__nutDebug(), event = new MouseEvent('mousemove');
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
        await page.evaluate(input => window.__nutInput = input, {x:dx/length,z:dz/length,sprint:false});
        await page.waitForTimeout(80);
      }
    } finally {await page.evaluate(() => window.__nutInput = {x:0,z:0,sprint:false});}
    await page.waitForTimeout(160);
  }
  const pets=()=>snapshot.nutIsland?.pets||[];
  async function menu(){await resume();await page.keyboard.press('i');await page.locator('#modal').waitFor({state:'visible'});}
  async function nutMenu(){await menu();await page.getByRole('button',{name:'Nut island',exact:true}).click();}
  async function selectPetItem(name){await menu();await page.getByRole('button',{name:new RegExp('^bag slot .*: '+name+', 1')}).click();}
  async function care(action,target){const button=page.locator('[data-action="'+action+'"]').filter({hasNot:page.locator('[disabled]')});for(const element of await button.all()){const payload=JSON.parse(await element.getAttribute('data-payload')||'{}');if(payload.target===target){await until(()=>element.isEnabled(),'pet control in reach');await element.click();return;}}throw Error('missing animal action '+action+' '+target);}
  async function approachPet(petID){await resume();let start=Date.now();while(true){const pet=pets().find(p=>p.id===petID);assert.ok(pet,'pet exists');const distance=Math.hypot(pet.x-snapshot.me.x,pet.z-snapshot.me.z);if(distance<2.4)break;assert.ok(Date.now()-start<20000,'walk to roaming companion');const dx=pet.x-snapshot.me.x,dz=pet.z-snapshot.me.z;await page.evaluate(input=>window.__nutInput=input,{x:dx/distance,z:dz/distance,sprint:false});await page.waitForTimeout(100);}await page.evaluate(()=>window.__nutInput={x:0,z:0,sprint:false});}

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
        if (message.type === 'input' && window.__nutInput) payload = JSON.stringify({...message,...window.__nutInput});
        return send.call(this,payload);
      };
    });
    await page.route('**/game.js', async route => {
      const response = await route.fetch(), source = await response.text(), anchor = 'function receiveSnapshot(snapshot){';
      assert.ok(source.includes(anchor), 'read-only nut island debug anchor');
      const debug = 'window.__nutDebug=()=>({yaw:state.yaw,pitch:state.pitch,locked:state.locked,camera:{x:camera.position.x,y:camera.position.y,z:camera.position.z},nutVisible:scene.getObjectByName("nut-garden-island")?.visible,animals:scene.getObjectByName("small-wildlife")?.children.map(o=>({name:o.name,x:o.position.x,y:o.position.y,z:o.position.z,visible:o.visible}))});';
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
            history.push({time:next.time,x:next.me.x,y:next.me.y,z:next.me.z,posture:next.me.posture,ridingCamel:next.me.ridingCamel,nutIsland:next.nutIsland});
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

    assert.equal(snapshot.me.inCave,true);assert.equal(snapshot.me.y,-24);
    await face(120,-126,-.12);await page.waitForFunction(()=>/green lollipop/i.test(document.querySelector('#interaction-label').textContent));await shot('green-lollipop');
    await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});await shot('green-island-offer');
    await page.getByRole('button',{name:'Unlock nut island · 1,000 coins',exact:true}).click();await until(()=>snapshot.me.nutIsland,'unlock permanent nut island');assert.equal(snapshot.me.coins,300);assert.equal(snapshot.me.island,'');
    await page.getByRole('button',{name:'Go to my nut island',exact:true}).click();await until(()=>snapshot.me.island==='nut:'+id,'arrive on owned nut island');await resume();await until(()=>page.evaluate(()=>window.__nutDebug().nutVisible),'nut scenery visible');await shot('nut-island-arrival');
    await selectPetItem('Squirrel');await page.getByRole('button',{name:'Release here · roam freely',exact:true}).click();await until(()=>pets().length===1,'inventory squirrel released');assert.equal(snapshot.me.inventory.pet_squirrel||0,0);const squirrelID=pets()[0].id;
    await resume();await face(pets()[0].x,pets()[0].z,-.35);await shot('squirrel-roaming');
    await travel(4,24);await face(4,22,-.08);await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});await page.getByRole('button',{name:'Build next cage · 150 coins',exact:true}).click();await until(()=>snapshot.nutIsland.cages.length===1,'buy physical cage');assert.equal(snapshot.me.coins,150);const cage=snapshot.nutIsland.cages[0];
    await approachPet(squirrelID);await nutMenu();await care('nut_pet_cage',squirrelID);await until(()=>pets().find(p=>p.id===squirrelID)?.cageId===cage.id,'assign roaming squirrel to cage');
    await travel(12,18);await face(cage.x,cage.z,-.15);await shot('roomy-animal-cage');await nutMenu();await care('nut_pet_free',squirrelID);await until(()=>!pets().find(p=>p.id===squirrelID)?.cageId,'let squirrel roam freely');
    await approachPet(squirrelID);await nutMenu();await care('nut_pet_pack',squirrelID);await until(()=>!pets().length&&snapshot.me.inventory.pet_squirrel===1,'pick squirrel back into inventory');
    await travel(12,18);await selectPetItem('Cactus mouse');await page.getByRole('button',{name:'Release into cage 1',exact:true}).click();await until(()=>pets().some(p=>p.species==='cactus_mouse'&&p.cageId===cage.id),'release held mouse into nearby cage');assert.equal(snapshot.me.inventory.pet_cactus_mouse||0,0);
    await resume();await face(cage.x,cage.z,-.25);await shot('cactus-mouse-in-cage');
    await nutMenu();await page.getByRole('button',{name:'Return to the Clay Grotto',exact:true}).click();await until(()=>snapshot.me.island===''&&snapshot.me.inCave,'return to cave');assert.equal(snapshot.me.y,-24);
    await menu();await page.getByRole('button',{name:'Teleport',exact:true}).click();await page.getByRole('button',{name:'Go to my nut island',exact:true}).click();await until(()=>snapshot.me.island==='nut:'+id,'reenter nut island');assert.equal(snapshot.nutIsland.cages.length,1);assert.equal(pets().filter(p=>p.species==='cactus_mouse').length,1);assert.equal(snapshot.me.inventory.pet_squirrel,1);assert.equal(snapshot.me.inventory.wood,17);assert.equal(snapshot.me.coins,150);
    await resume();await shot('nut-island-return-visit');
    const playerFile=path.join(data,'players',id+'.json');await until(()=>{try{const p=JSON.parse(fs.readFileSync(playerFile,'utf8')).profile;return p.coins===150&&p.nutIsland?.cages.length===1&&p.nutIsland?.pets.length===1&&p.inventory.pet_squirrel===1;}catch{return false;}},'island, cage, pet and inventory persisted');
    await page.reload({waitUntil:'networkidle'});await page.locator('#start-button').waitFor({state:'visible'});await page.locator('#start-button').click();await until(()=>snapshot.me.nutIsland?.cages?.length===1,'reconnect restores owned island');
    if(snapshot.me.island!=='nut:'+id){await menu();await page.getByRole('button',{name:'Teleport',exact:true}).click();await page.getByRole('button',{name:'Go to my nut island',exact:true}).click();}await until(()=>snapshot.me.island==='nut:'+id&&pets().length===1,'reconnect island pet restored');await resume();await shot('nut-island-after-reconnect');
    assert.equal(snapshot.me.coins,150);assert.equal(snapshot.me.inventory.pet_squirrel,1);assert.equal(snapshot.me.inventory.wood,17);assert.deepEqual(errors,[]);assert.deepEqual(failed,[]);
    console.log('PASS: green cave unlock, real inventory release, 150-coin cage, assign/free/pack/release into cage, return/reentry/reconnect and persistent animal inventory.');
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
    fs.writeFileSync(path.join(out, 'island-history.json'),JSON.stringify(history,null,2));
  }
})().catch(error => {console.error(error);process.exitCode = 1;});
