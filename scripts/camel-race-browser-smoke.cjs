// Exercise the real Go race, rendering, menus and websocket movement in Chrome.
// Only the isolated initial clock and login spawn are changed by build overlays.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const net=require('node:net');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results','camel-race-browser-'+Date.now()),data=path.join(out,'data');
  fs.mkdirSync(data,{recursive:true});
  const email='camel-race@example.com',id='u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32),camelID=id+'_camel_1';
  const slots=n=>Array.from({length:n},()=>({item:'',count:0}));
  fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:3,profiles:{[id]:{
    id,name:'Race Explorer',callName:'Race Explorer',username:'race_explorer',coins:1000,
    inventoryVersion:3,hotbar:slots(5),bagSlots:slots(20),selectedSlot:0,levels:{},
    camels:[{id:camelID,name:'Apricot',x:-224,y:0,z:20,yaw:Math.PI/2,island:'',inCave:false,rentalUntil:0,speed:15,jump:8,stamina:170,happiness:100,traitsVersion:1}]
  }}}));
  const sourceFile=path.join(root,'internal/game/world.go'),source=fs.readFileSync(sourceFile,'utf8');
  const changed=source.replace('\tw.seedMap()','\tw.time = 60\n\tw.seedMap()')
    .replace(/(\tw\.spawn\(p\)\r?\n)(\tw\.players\[id\] = p)/,'$1\tp.x, p.z, p.yaw = -211, 21, math.Pi/2\n$2');
  assert.notEqual(source,changed,'overlay anchors');
  const overlaySource=path.join(out,'world.go.txt');fs.writeFileSync(overlaySource,changed);
  const overlayFile=path.join(out,'overlay.json');fs.writeFileSync(overlayFile,JSON.stringify({Replace:{[sourceFile]:overlaySource}}));
  const executable=path.join(out,process.platform==='win32'?'server.exe':'server');
  const go=process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go');
  const build=spawnSync(go,['build','-overlay',overlayFile,'-o',executable,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});
  assert.equal(build.status,0,build.stderr||build.stdout);
  const port=await new Promise((resolve,reject)=>{const probe=net.createServer();probe.once('error',reject);probe.listen(0,'127.0.0.1',()=>{const selected=probe.address().port;probe.close(error=>error?reject(error):resolve(selected));});});
  const base='http://127.0.0.1:'+port;
  let browser,page,snapshot,activeSocket,log='';
  const errors=[],failed=[],actionErrors=[],history=[];
  const server=spawn(executable,['-dev','-addr','127.0.0.1:'+port,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});
  server.stdout.on('data',chunk=>log+=chunk);server.stderr.on('data',chunk=>log+=chunk);
  const {createSnapshotDecoder}=await import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,'web/state-sync.js'),'utf8')).toString('base64'));
  async function until(check,label,timeout=20000){const start=Date.now();while(!await check()){assert.ok(Date.now()-start<timeout,label+'; '+[...errors,...actionErrors].join('; '));await new Promise(resolve=>setTimeout(resolve,80));}}
  const shot=name=>page.screenshot({path:path.join(out,name+'.png')});
  async function resume(){if(await page.locator('#modal').isVisible())await page.locator('#close-modal').click();if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement&&window.__raceDebug().locked);}
  async function face(x,z){const yaw=Math.atan2(-(x-snapshot.me.x),-(z-snapshot.me.z));await page.evaluate(yaw=>{const current=window.__raceDebug(),event=new MouseEvent('mousemove');Object.defineProperty(event,'movementX',{value:(current.yaw-yaw)/.002});Object.defineProperty(event,'movementY',{value:(current.pitch+.03)/.002});document.dispatchEvent(event);},yaw);await page.waitForTimeout(150);}
  const own=()=>snapshot?.camelRace?.entrants?.find(e=>e.id===id);
  try{
    await until(()=>fetch(base+'/healthz').then(r=>r.ok).catch(()=>false),'server start');
    browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    page=await browser.newPage({viewport:{width:1440,height:1000}});
    await page.addInitScript(()=>{const send=WebSocket.prototype.send;window.__raceSend=(message)=>send.call(window.__raceSocket,JSON.stringify(message));WebSocket.prototype.send=function(payload){window.__raceSocket=this;if(window.__raceDriving&&JSON.parse(payload).type==='input')return;return send.call(this,payload);};});
    await page.route('**/game.js',async route=>{
      const response=await route.fetch(),source=await response.text();assert.ok(source.includes('function receiveSnapshot(snapshot){'));
      await route.fulfill({response,body:source.replace('function receiveSnapshot(snapshot){',
        "window.__raceDebug=()=>{const visible=o=>{if(!o)return false;for(let p=o;p;p=p.parent)if(!p.visible)return false;return true;};return {yaw:state.yaw,pitch:state.pitch,locked:state.locked,snapshot:state.snapshot,worldVisible:worldGroup.visible,ringVisible:visible(scene.getObjectByName('camel-racing-ring')),homeVisible:visible(scene.getObjectByName('return-beacon')),caveVisible:visible(scene.getObjectByName('beach-race-cliff'))};};function receiveSnapshot(snapshot){")
        .replace('renderer.render(scene,camera);','window.__raceRender=(window.__raceRender||0)+1;if(window.__raceRender%3===0)renderer.render(scene,camera);')});
    });
    page.on('pageerror',error=>errors.push(error.message));
    page.on('response',response=>{if(response.status()>=400)failed.push(response.status()+' '+response.url());});
    page.on('websocket',socket=>{activeSocket=socket;const decoder=createSnapshotDecoder();socket.on('framereceived',({payload})=>{if(socket!==activeSocket)return;try{const message=JSON.parse(payload.toString()),next=decoder.apply(message);if(next){snapshot=next;history.push({time:next.time,x:next.me.x,z:next.me.z,island:next.me.island,coins:next.me.coins,camelCoins:next.me.camelCoins,race:own()});}if(message.type==='error')actionErrors.push(message.message);}catch(error){errors.push(error.message);}});});
    await page.goto(base,{waitUntil:'networkidle'});
    await page.locator('#email').fill(email);await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});
    if(!await page.locator('#login-code').inputValue())await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await until(()=>snapshot?.me,'initial snapshot');
    await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);
    await face(-227,20);await shot('cliff-entrance');
    await page.keyboard.down('w');await until(()=>snapshot.me.x<-221,'walk through east entrance',7000);await page.keyboard.up('w');
    await page.evaluate(target=>window.__raceSend({type:'action',action:'camel_mount',target}),camelID);
    await until(()=>snapshot.me.ridingCamel,'mount Apricot');
    const origin={x:snapshot.me.x,z:snapshot.me.z};
    assert.equal(snapshot.me.inCave,false);assert.equal(snapshot.me.y,0);
    await face(-227,20);await page.waitForFunction(()=>/camel lollipop/i.test(document.querySelector('#interaction-label').textContent));
    await shot('camel-lollipop-inside-cliff');await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});
    await page.getByRole('button',{name:'Eat camel lollipop & enter'}).click();
    await until(()=>snapshot.camelRace.registeredStartAt===120,'registration');
    await shot('race-entry-menu');await resume();
    console.log('Entered at server time '+snapshot.time.toFixed(1)+'; awaiting the real 20-second starting grid.');
    await until(()=>snapshot.camelRace.inArena,'gather teleport',50000);
    const parked=structuredClone(snapshot.me.camels[0]);
    assert.equal(snapshot.camelRace.phase,'gathering');assert.equal(snapshot.me.island,'camel_race');assert.equal(snapshot.me.ridingCamel,true);
    assert.equal(snapshot.camelRace.ownCamel.id,camelID);assert.equal(snapshot.camelRace.ownCamel.speed,15);assert.equal(snapshot.me.maxStamina,170);
    assert.equal(snapshot.nodes.length,0);assert.equal(snapshot.island,undefined);
    await until(()=>page.evaluate(()=>{const d=window.__raceDebug();return d.ringVisible&&!d.worldVisible&&!d.homeVisible;}),'isolated ring scenery');
    const grid={x:snapshot.me.x,z:snapshot.me.z};await page.keyboard.down('w');await page.waitForTimeout(700);await page.keyboard.up('w');
    assert.deepEqual({x:snapshot.me.x,z:snapshot.me.z},grid,'countdown freezes player');
    await shot('race-grid-countdown');await page.keyboard.press('r');await page.locator('#modal').waitFor({state:'visible'});
    assert.match(await page.locator('#modal-content').innerText(),/starting grid/);await shot('race-countdown-menu');await resume();
    await until(()=>snapshot.camelRace.phase==='racing','actual race start',25000);
    const coins=snapshot.me.coins,camelCoins=snapshot.me.camelCoins;
    await page.evaluate(()=>{
      window.__raceDriving=true;
      window.__raceDriver=setInterval(()=>{
        const d=window.__raceDebug(),s=d.snapshot,m=s?.me,e=s?.camelRace?.entrants?.find(e=>e.id===m.id);
        if(!m||m.island!=='camel_race'||e?.status!=='racing')return;
        const a=Math.atan2(m.z/33.5,m.x/51.5)+.14,x=51.5*Math.cos(a)-m.x,z=33.5*Math.sin(a)-m.z,length=Math.hypot(x,z),yaw=Math.atan2(-x,-z);
        const event=new MouseEvent('mousemove');Object.defineProperty(event,'movementX',{value:(d.yaw-yaw)/.002});document.dispatchEvent(event);
        window.__raceSend({type:'input',x:x/length,z:z/length,yaw,pitch:0,sprint:!m.sprintExhausted&&m.stamina>12,jump:false});
      },50);
    });
    await until(()=>own()?.lap>=1,'first complete lap',45000);await shot('race-lap-two');
    console.log('Completed first lap; continuing real movement through all 48 ordered gates.');
    await until(()=>own()?.status==='finished','three-lap finish',110000);
    await page.evaluate(()=>{clearInterval(window.__raceDriver);window.__raceDriving=false;window.__raceSend({type:'input',x:0,z:0,yaw:window.__raceDebug().yaw,pitch:0});});
    assert.equal(own().lap,3);assert.equal(own().rank,1);assert.equal(snapshot.me.coins,coins);assert.equal(snapshot.me.camelCoins,camelCoins+50);
    assert.equal(own().coins,0);assert.equal(own().camelCoins,50);assert.deepEqual(snapshot.me.camels[0],parked,'race did not move owned camel parking');
    await shot('race-finish-prize');await page.keyboard.press('r');await page.locator('#modal').waitFor({state:'visible'});
    assert.match(await page.locator('#modal-content').innerText(),/Finished in place 1/);
    await page.getByRole('button',{name:'Return to the beach'}).click();await until(()=>snapshot.me.island==='','return to cliff');
    assert.deepEqual({x:snapshot.me.x,z:snapshot.me.z},origin);assert.equal(snapshot.me.mountedCamelId,camelID);assert.equal(snapshot.me.ridingCamel,true);
    await resume();await until(()=>page.evaluate(()=>{const d=window.__raceDebug();return !d.ringVisible&&d.worldVisible&&!d.homeVisible;}),'mainland restored');await shot('returned-with-owned-camel');
    assert.deepEqual(errors,[]);assert.deepEqual(failed,[]);assert.deepEqual(actionErrors,[]);
    console.log('PASS: dry cliff entry, lollipop registration, gather/countdown, isolated ring, owned traits, authoritative three-lap drive, one solo payout, previous position and camel restored.');
    console.log('Screenshots: '+out);
  }catch(error){if(page)await shot('failure').catch(()=>{});console.error({errors,actionErrors,failed,me:snapshot?.me,race:snapshot?.camelRace});throw error;}
  finally{await browser?.close();server.kill();if(server.exitCode===null)await Promise.race([new Promise(resolve=>server.once('exit',resolve)),new Promise(resolve=>setTimeout(resolve,3000))]);fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'history.json'),JSON.stringify(history));}
})().catch(error=>{console.error(error);process.exitCode=1;});
