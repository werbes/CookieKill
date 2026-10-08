// Isolated Go + real Chrome: a full tree climb, cookie arena PvP, both kill boards and footsteps.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),net=require('node:net');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));
(async()=>{
  const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results','cookie-ring-browser-'+Date.now()),data=path.join(out,'data'),treeOnly=process.argv.includes('--tree-only');fs.mkdirSync(data,{recursive:true});
  const identity=email=>'u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32),email='cookie-ring@example.com',friendEmail='cookie-rival@example.com',id=identity(email),friendID=identity(friendEmail),slots=n=>Array.from({length:n},()=>({item:'',count:0}));
  const profile=(id,name)=>({id,name,callName:name,username:name.toLowerCase().replaceAll(' ','_'),coins:500,inventoryVersion:3,hotbar:slots(5),bagSlots:slots(20),levels:{},camels:[]});
  const actor=profile(id,'Canopy Explorer'),friend=profile(friendID,'Rival');actor.kills=4;actor.cookieRingKills=2;actor.bagSlots[0]={item:'flower_bouquet',count:1};
  fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:3,profiles:{[id]:actor,[friendID]:friend,old_gardener:{...profile('old_gardener','Garden Legend'),camelCoins:80,coinsEarned:200,playSeconds:7200,kills:7}}}));
  const sourceFile=path.join(root,'internal/game/world.go'),source=fs.readFileSync(sourceFile,'utf8'),changed=source.replace(/(\tw\.spawn\(p\)\r?\n)(\tw\.players\[id\] = p)/,'$1\tp.x, p.y, p.z = 207, 0, 198\n\tif id == "'+friendID+'" { p.x, p.y, p.z = 214, 12, 178 }\n$2');assert.notEqual(changed,source,'test spawn overlay anchor');
  const worldOverlay=path.join(out,'world.txt'),overlay=path.join(out,'overlay.json');fs.writeFileSync(worldOverlay,changed);fs.writeFileSync(overlay,JSON.stringify({Replace:{[sourceFile]:worldOverlay}}));
  const executable=path.join(out,process.platform==='win32'?'server.exe':'server'),go=process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go');
  const build=spawnSync(go,['build','-overlay',overlay,'-o',executable,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});assert.equal(build.status,0,build.stderr||build.stdout);
  const port=await new Promise((resolve,reject)=>{const probe=net.createServer();probe.once('error',reject);probe.listen(0,'127.0.0.1',()=>{const chosen=probe.address().port;probe.close(()=>resolve(chosen));});}),base='http://127.0.0.1:'+port;
  let log='',browser,page,snapshot,friendSnapshot;const errors=[],failed=[],actions=[];
  const server=spawn(executable,['-dev','-addr','127.0.0.1:'+port,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});server.stdout.on('data',chunk=>log+=chunk);server.stderr.on('data',chunk=>log+=chunk);
  const {createSnapshotDecoder}=await import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,'web/state-sync.js'),'utf8')).toString('base64'));
  async function until(fn,label,timeout=18000){const start=Date.now();while(!await fn()){assert.ok(Date.now()-start<timeout,label+'; '+errors.join('; '));await new Promise(resolve=>setTimeout(resolve,80));}}
  const shot=name=>page.screenshot({path:path.join(out,name+'.png')});
  async function resume(){if(await page.locator('#modal').isVisible())await page.locator('#close-modal').click();if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement&&window.__townDebug().locked);}
  async function travel(x,z){await resume();const start=Date.now();try{while(Math.hypot(snapshot.me.x-x,snapshot.me.z-z)>.35){assert.ok(Date.now()-start<18000,'route blocked toward '+x+','+z+' from '+snapshot.me.x+','+snapshot.me.z);const dx=x-snapshot.me.x,dz=z-snapshot.me.z,d=Math.max(1,Math.hypot(dx,dz));await page.evaluate(input=>window.__townMovement=input,{x:dx/d,z:dz/d,sprint:false});await page.waitForTimeout(75);}}finally{await page.evaluate(()=>window.__townMovement={x:0,z:0,sprint:false});}await page.waitForTimeout(150);}
  async function face(x,z,pitch=-.05){await resume();const yaw=Math.atan2(-(x-snapshot.me.x),-(z-snapshot.me.z));await page.evaluate(({yaw,pitch})=>{const d=window.__townDebug(),event=new MouseEvent('mousemove');Object.defineProperty(event,'movementX',{value:(d.yaw-yaw)/.002});Object.defineProperty(event,'movementY',{value:(d.pitch-pitch)/.002});document.dispatchEvent(event);},{yaw,pitch});await page.waitForTimeout(180);}
  async function interact(text){await page.waitForFunction(text=>document.getElementById('interaction-label').textContent.includes(text),text);await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});}
  async function setupPage(email,secondary=false){
    const context=await browser.newContext({viewport:{width:1440,height:1000}}),tab=await context.newPage();
    await tab.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){try{const message=JSON.parse(payload);if(message.type==='input'&&window.__townMovement)payload=JSON.stringify({...message,...window.__townMovement});}catch{}return send.call(this,payload);};});
    await tab.route('**/game.js',async route=>{const response=await route.fetch(),source=await response.text();await route.fulfill({response,body:source.replace('const ambientAudio=createAmbientAudio();',"const ambientAudio=createAmbientAudio({onEvent:event=>{if(event.type==='footstep')(window.__footsteps??=[]).push(event);}});").replace('function receiveSnapshot(snapshot){','window.__townDebug=()=>({yaw:state.yaw,pitch:state.pitch,locked:state.locked,snapshot:state.snapshot,worldVisible:worldGroup.visible,ringVisible:treeRingScene?.ring.visible,treeVisible:treeRingScene?.tree.visible,heldCookie:hand?.getObjectByName("held-cookie")?.children.length});function receiveSnapshot(snapshot){').replace('renderer.render(scene,camera);','window.__townFrames=(window.__townFrames||0)+1;if(window.__townFrames%'+(secondary?18:4)+'===0)renderer.render(scene,camera);')});});
    tab.on('pageerror',error=>errors.push(error.message));tab.on('response',response=>{if(response.status()>=400)failed.push(response.status()+' '+response.url());});
    tab.on('websocket',socket=>{const decoder=createSnapshotDecoder();socket.on('framereceived',({payload})=>{try{const message=JSON.parse(payload.toString()),next=decoder.apply(message);if(next){if(secondary)friendSnapshot=next;else snapshot=next;}if(message.type==='error')errors.push(message.message);}catch(error){errors.push(error.message);}});socket.on('framesent',({payload})=>{const message=JSON.parse(payload.toString());if(message.type==='action')actions.push(message);});});
    await tab.goto(base,{waitUntil:'networkidle'});await tab.locator('#email').fill(email);await tab.locator('#email-submit').click();await tab.locator('#code-stage').waitFor({state:'visible'});
    if(!await tab.locator('#login-code').inputValue())await tab.locator('#login-code').fill((await tab.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);await tab.locator('#code-submit').click();await tab.locator('#start-button').waitFor({state:'visible'});return tab;
  }
  try{
    await until(()=>fetch(base+'/healthz').then(response=>response.ok).catch(()=>false),'server start');browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    const friendPage=treeOnly?null:await setupPage(friendEmail,true);if(friendPage)await until(()=>friendSnapshot?.me,'friend connected');
    page=await setupPage(email);await until(()=>snapshot?.me,'explorer connected');await page.locator('#start-button').click();await resume();

    assert.equal(snapshot.town.leaderboards.length,5);assert.equal(snapshot.layout.props.some(p=>p.kind==='gym'),false,'old city gym moved');
    await face(210,184,.35);await shot('south-tree-exterior');
    await travel(207.6,188.5);await face(207.4,186.2,.2);await shot('outside-spiral-stair-entrance');
    const stairPoint=i=>{const a=13*Math.PI/16+(i+.5)*2*Math.PI/32;return {x:210+Math.cos(a)*3.4,z:184+Math.sin(a)*3.4};};
    for(let i=0;i<64;i+=2){const p=stairPoint(i);await face(p.x,p.z,.12);await travel(p.x,p.z);if(i===32){assert.ok(Math.abs(snapshot.me.y-6)<.5,'first exterior turn reaches halfway');await shot('halfway-around-slim-trunk');}}
    const top=stairPoint(63);await travel(top.x,top.z);await travel(207.6,185.5);assert.ok(Math.abs(snapshot.me.y-12)<.05,'exterior spiral reaches hut bridge');await face(207.6,180,0);await shot('spiral-bridge-to-unchanged-hut');
    await travel(207.6,181);await travel(212,178);await face(213,177.5,0);await shot('treehut-golden-cookie');
    const origin={x:snapshot.me.x,y:snapshot.me.y,z:snapshot.me.z},inventory=structuredClone(snapshot.me.inventory),coins=snapshot.me.coins;
    await interact('golden cookie');await page.getByRole('button',{name:/Eat golden cookie/}).click();await until(()=>snapshot.me.island==='cookie_ring','golden teleport');
    await until(()=>page.evaluate(()=>{const d=window.__townDebug();return d.ringVisible&&!d.worldVisible&&d.heldCookie>0;}),'arena scenery and loan cookie');await resume();await shot('cookie-fighting-ring');
    if(treeOnly){
      await page.keyboard.press('r');await page.getByRole('button',{name:'Return to the tree hut'}).click();await until(()=>snapshot.me.island==='','ring return to unchanged room');await resume();
      await travel(214,181);await face(215,181.5,0);await shot('treehut-silver-cookie');await interact('silver cookie');await page.getByRole('button',{name:/Eat silver cookie/}).click();await until(()=>snapshot.me.island==='cookie_practice','silver cookie still reachable');await resume();
      await page.keyboard.press('r');await page.getByRole('button',{name:'Return to the tree hut'}).click();await until(()=>snapshot.me.island==='','practice return to room');await resume();
      await travel(207.6,181);await travel(207.6,185.5);
      for(let i=63;i>=0;i-=2){const p=stairPoint(i);await face(p.x,p.z,-.18);await travel(p.x,p.z);}
      const bottom=stairPoint(0);await travel(bottom.x,bottom.z);await travel(207.6,188.5);assert.equal(snapshot.me.y,0,'normal walking descends spiral to ground');await travel(207,198);await face(210,184,.35);await shot('slim-trunk-after-full-descent');
      assert.deepEqual(snapshot.me.inventory,inventory);assert.equal(snapshot.me.coins,coins);assert.deepEqual(errors,[]);assert.deepEqual(failed,[]);
      console.log('PASS: exterior spiral climb and descent, slim solid trunk, same-size hut, golden/silver portal access and return, unchanged inventory.');console.log('Screenshots: '+out);return;
    }
    const ownerPage=page;page=friendPage;await page.locator('#start-button').click();await resume();await interact('golden cookie');await page.getByRole('button',{name:/Eat golden cookie/}).click();await until(()=>friendSnapshot.me.island==='cookie_ring','rival enters ring');
    page=ownerPage;await page.bringToFront();await resume();await until(()=>snapshot.players.some(p=>p.id===friendID),'rival visible');
    await travel(-20,17);await travel(20,17);await travel(20,9);
    await friendPage.evaluate(()=>window.__townMovement={x:-1,z:0,sprint:false});await page.waitForTimeout(600);await friendPage.evaluate(()=>window.__townMovement={x:1,z:0,sprint:false});await page.waitForTimeout(600);await friendPage.evaluate(()=>window.__townMovement={x:0,z:0,sprint:false});await page.waitForTimeout(350);
    assert.ok(await page.evaluate(()=>window.__footsteps?.some(e=>!e.local)),'nearby player walking produces audible footsteps');
    await face(friendSnapshot.me.x,friendSnapshot.me.z,.045);await shot('aiming-at-ring-rival');
    const kills=snapshot.me.kills,ringKills=snapshot.me.cookieRingKills,deaths=friendSnapshot.me.deaths||0;
    for(let i=0;i<8&&snapshot.me.cookieRingKills===ringKills;i++){await page.mouse.down();await page.mouse.up();await page.waitForTimeout(730);}
    await until(()=>snapshot.me.cookieRingKills===ringKills+1,'real cookie projectile kill',5000);assert.equal(snapshot.me.kills,kills+1);assert.equal(friendSnapshot.me.deaths,deaths+1);assert.equal(friendSnapshot.me.island,'cookie_ring');assert.equal(friendSnapshot.me.health,100);assert.equal(snapshot.cookieArena.kills,1);
    assert.equal(snapshot.town.leaderboards.find(b=>b.id==='kills').self.value,kills+1);assert.equal(snapshot.town.leaderboards.find(b=>b.id==='cookie_ring_kills').self.value,ringKills+1);
    assert.equal(snapshot.me.coins,coins);assert.deepEqual(snapshot.me.inventory,inventory);assert.equal(snapshot.me.hats.champion,true);await shot('ring-victory');
    await page.keyboard.press('r');await page.locator('#modal').waitFor({state:'visible'});assert.match(await page.locator('#modal-content').innerText(),/1 ring wins/);await shot('ring-score-and-return');await page.getByRole('button',{name:'Return to the tree hut'}).click();await until(()=>snapshot.me.island==='','return to high hut');
    assert.deepEqual({x:snapshot.me.x,y:snapshot.me.y,z:snapshot.me.z},origin);await resume();await page.waitForTimeout(600);assert.ok(Math.abs(snapshot.me.y-12)<.05,'returned hut support remains solid');
    await travel(214,181);await face(215,181.5,0);await shot('treehut-silver-cookie');await interact('silver cookie');await page.getByRole('button',{name:/Eat silver cookie/}).click();await until(()=>snapshot.me.island==='cookie_practice','silver teleport');
    assert.equal(snapshot.players.some(p=>p.id===friendID),false,'ring and practice are separate worlds');assert.equal(snapshot.cookiePractice.stations.length,4);await resume();await face(-8,-21,0);await shot('silver-practice-center');
    const localSteps=await page.evaluate(()=>window.__footsteps||[]);assert.ok(localSteps.some(e=>e.local),'actual walking produces quiet footsteps');
    await page.keyboard.press('r');await page.getByRole('button',{name:'Return to the tree hut'}).click();await until(()=>snapshot.me.island==='','practice return');
    assert.deepEqual(errors,[]);assert.deepEqual(failed,[]);
    console.log('PASS: two exterior spiral turns around a slim trunk, unchanged 12m-high hut, golden/silver cookies, isolated ring, two-player cookie kill, both persisted counters, ring respawn, unchanged wallet/inventory, treehut return and actual footstep scheduling.');console.log('Screenshots: '+out);
  }catch(error){if(page)await shot('failure').catch(()=>{});console.error({errors,failed,me:snapshot?.me});throw error;}
  finally{await browser?.close();server.kill();if(server.exitCode===null)await Promise.race([new Promise(resolve=>server.once('exit',resolve)),new Promise(resolve=>setTimeout(resolve,3000))]);fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actions,null,2));}
})().catch(error=>{console.error(error);process.exitCode=1;});
