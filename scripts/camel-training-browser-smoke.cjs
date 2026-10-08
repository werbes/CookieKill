// Exercise actual camel training movement, boost accounting, rendering and return in Chrome.
// Only the isolated initial clock and login spawn are changed by build overlays.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const net=require('node:net');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results','camel-training-browser-'+Date.now()),data=path.join(out,'data');
  fs.mkdirSync(data,{recursive:true});
  const email='camel-training@example.com',id='u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32),camelID=id+'_camel_1';
  const slots=n=>Array.from({length:n},()=>({item:'',count:0}));
  fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:3,profiles:{[id]:{
    id,name:'Race Explorer',callName:'Race Explorer',username:'race_explorer',coins:1000,
    inventoryVersion:3,hotbar:[{item:'swift_dates',count:3},...slots(4)],bagSlots:slots(20),selectedSlot:0,levels:{},
    camels:[{id:camelID,name:'Apricot',x:-224,y:0,z:20,yaw:Math.PI/2,island:'',inCave:false,rentalUntil:0,speed:10,jump:8,stamina:170,happiness:80,traitsVersion:1}]
  }}}));
  const sourceFile=path.join(root,'internal/game/world.go'),source=fs.readFileSync(sourceFile,'utf8');
  const changed=source.replace('\tw.seedMap()','\tw.time = 60\n\tw.seedMap()')
    .replace(/(\tw\.spawn\(p\)\r?\n)(\tw\.players\[id\] = p)/,'$1\tp.x, p.z, p.yaw = -229, 14, math.Pi/2\n$2');
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
        "window.__raceDebug=()=>{const visible=o=>{if(!o)return false;for(let p=o;p;p=p.parent)if(!p.visible)return false;return true;};return {yaw:state.yaw,pitch:state.pitch,locked:state.locked,snapshot:state.snapshot,worldVisible:worldGroup.visible,ringVisible:visible(scene.getObjectByName('camel-racing-ring')),homeVisible:visible(scene.getObjectByName('return-beacon')),caveVisible:visible(scene.getObjectByName('beach-race-cliff')),trainingVisible:visible(scene.getObjectByName('camel-training-gardens')),trainingGroup:scene.getObjectByName('camel-training-gardens')?.name};};function receiveSnapshot(snapshot){")
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

    await face(-231,14);await page.waitForFunction(()=>/pink camel/i.test(document.querySelector('#interaction-label').textContent));
    await shot('pink-camel-lollipop');await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});
    assert.match(await page.locator('#modal-content').innerText(),/Training is free/);
    const origin={x:snapshot.me.x,z:snapshot.me.z},parked=structuredClone(snapshot.me.camels[0]);
    await page.getByRole('button',{name:/Eat pink lollipop/}).click();await until(()=>snapshot.me.island==='camel_training','enter training');
    assert.equal(snapshot.me.ridingCamel,true);assert.equal(snapshot.camelTraining.ownCamel.id,camelID);
    await until(()=>page.evaluate(()=>{const d=window.__raceDebug();return d.trainingVisible&&!d.worldVisible&&!d.ringVisible&&!d.homeVisible;}),'isolated training scenery');
    await resume();await page.waitForFunction(()=>Math.abs(window.__raceDebug().yaw-Math.PI)<.01);await page.waitForTimeout(200);await shot('speed-training-course');await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});
    await page.getByRole('button',{name:/Swift dates/}).click();await until(()=>snapshot.camelTraining.ownCamel.trainingBoost.speed===120,'treat boost');
    assert.equal(snapshot.camelTraining.ownCamel.speed,10);assert.equal(snapshot.me.inventory.swift_dates,2);
    await shot('training-progress-menu');await resume();await page.waitForTimeout(1500);
    assert.equal(snapshot.camelTraining.ownCamel.speed,10);assert.equal(snapshot.camelTraining.ownCamel.training.speed,0,'idle never trains');
    const startZ=snapshot.me.z;await page.keyboard.down('w');await until(()=>snapshot.me.z>startZ+2,'W follows the course after arriving',3000);await page.keyboard.up('w');
    await page.evaluate(()=>{
      window.__raceDriving=true;let leg=0;
      window.__raceDriver=setInterval(()=>{
        const d=window.__raceDebug(),s=d.snapshot,m=s?.me,t=s?.camelTraining,c=t?.courses?.find(c=>c.id===t.exercise);if(!m||m.island!=='camel_training'||!c)return;
        if(t.completedLaps>=2){window.__raceSend({type:'input',x:0,z:0,yaw:d.yaw,pitch:0});return;}
        let point=c.checkpoints[leg%4],x=point.x-m.x,z=point.z-m.z,length=Math.hypot(x,z);
        if(length<.7){leg++;point=c.checkpoints[leg%4];x=point.x-m.x;z=point.z-m.z;length=Math.hypot(x,z);}
        const yaw=Math.atan2(-x,-z),event=new MouseEvent('mousemove');Object.defineProperty(event,'movementX',{value:(d.yaw-yaw)/.002});document.dispatchEvent(event);
        window.__raceSend({type:'input',x:x/length,z:z/length,yaw,pitch:0,sprint:false,jump:false});
      },50);
    });
    await until(()=>snapshot.camelTraining.completedLaps>=1,'first real lap',35000);assert.equal(snapshot.camelTraining.ownCamel.speed,10);assert.equal(snapshot.camelTraining.ownCamel.training.speed,50);
    await shot('training-first-lap');await until(()=>snapshot.camelTraining.completedLaps>=2,'second real lap stat gain',35000);
    await page.evaluate(()=>{clearInterval(window.__raceDriver);window.__raceDriving=false;window.__raceSend({type:'input',x:0,z:0,yaw:window.__raceDebug().yaw,pitch:0});});
    assert.equal(snapshot.camelTraining.ownCamel.speed,10.1);assert.equal(snapshot.camelTraining.ownCamel.training.speed,0);assert.equal(snapshot.camelTraining.ownCamel.trainingBoost.speed,70);
    assert.deepEqual({x:snapshot.me.camels[0].x,z:snapshot.me.camels[0].z},{x:parked.x,z:parked.z});
    for(const exercise of ['jump','stamina','happiness']){
      await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});
      await page.locator('[data-action="training_select"]').evaluateAll((buttons,exercise)=>buttons.find(b=>JSON.parse(b.dataset.payload).item===exercise).click(),exercise);
      await until(()=>snapshot.camelTraining.exercise===exercise,'choose '+exercise);await resume();await shot(exercise+'-course');
    }
    await page.keyboard.press('m');await page.locator('#world-map').waitFor();await shot('training-world-map');await resume();
    await page.keyboard.press('e');await page.getByRole('button',{name:'Return to Camel Cove'}).click();await until(()=>snapshot.me.island==='','return from training');
    assert.deepEqual({x:snapshot.me.x,z:snapshot.me.z},origin);assert.equal(snapshot.me.ridingCamel,false);assert.equal(snapshot.me.camels[0].speed,10.1);assert.equal(snapshot.camelTraining.ownCamel,undefined);
    await resume();await until(()=>page.evaluate(()=>{const d=window.__raceDebug();return !d.trainingVisible&&d.worldVisible;}),'mainland restored');await shot('returned-from-training');
    assert.deepEqual(errors,[]);assert.deepEqual(failed,[]);assert.deepEqual(actionErrors,[]);
    console.log('PASS: pink portal menu, own camel, isolated four-course grounds, no idle/treat-only stat gain, two actual boosted laps, permanent gain, exercise changes, map and original beach pose restored.');console.log('Screenshots: '+out);
  }catch(error){if(page)await shot('failure').catch(()=>{});console.error({errors,actionErrors,failed,me:snapshot?.me,training:snapshot?.camelTraining});throw error;}
  finally{await browser?.close();server.kill();if(server.exitCode===null)await Promise.race([new Promise(resolve=>server.once('exit',resolve)),new Promise(resolve=>setTimeout(resolve,3000))]);fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'history.json'),JSON.stringify(history));}
})().catch(error=>{console.error(error);process.exitCode=1;});
