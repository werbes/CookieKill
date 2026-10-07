// Isolated real Chrome walk through the forest club, bar, seats and coin wheel.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),net=require('node:net');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));
(async()=>{
 const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results','club-browser-'+Date.now()),data=path.join(out,'data');fs.mkdirSync(data,{recursive:true});
 const email='club-review@example.com',id='u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32),slots=n=>Array.from({length:n},()=>({item:'',count:0})),hotbar=slots(5);hotbar[0]={item:'sugar',count:10};
 fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:2,profiles:{[id]:{id,name:'Club Explorer',callName:'Club Explorer',username:'club_explorer',coins:2000,inventoryVersion:2,safeSlots:slots(3),bagSlots:slots(15),hotbar,selectedSlot:0,levels:{},friendCode:'FERN1837',friends:{},friendRequests:{},avatar:{skin:1,shirt:'tee',pants:'trousers',shirtColor:'teal',pantsColor:'sand'},hats:{},discovered:{sugar:true},homeIsland:{owner:id,visitors:false,builders:{},chest:{},objects:[]}}}}));
 const exe=path.join(out,process.platform==='win32'?'server.exe':'server'),go=process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go');
 const build=spawnSync(go,['build','-o',exe,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});assert.equal(build.status,0,build.stderr||build.stdout);
 const port=await new Promise((resolve,reject)=>{const probe=net.createServer();probe.once('error',reject);probe.listen(0,'127.0.0.1',()=>{const port=probe.address().port;probe.close(()=>resolve(port));});}),base='http://127.0.0.1:'+port;
 let log='',browser,page,snapshot,activeSocket;const errors=[],failed=[],actions=[];
 const server=spawn(exe,['-dev','-addr','127.0.0.1:'+port,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});server.stdout.on('data',x=>log+=x);server.stderr.on('data',x=>log+=x);
 const decoderSource=fs.readFileSync(path.join(root,'web/state-sync.js'),'utf8'),{createSnapshotDecoder}=await import('data:text/javascript;base64,'+Buffer.from(decoderSource).toString('base64'));
 async function until(fn,label,timeout=20000){const begin=Date.now();while(!await fn()){assert.ok(Date.now()-begin<timeout,label);await new Promise(r=>setTimeout(r,90));}}
 const shot=name=>page.screenshot({path:path.join(out,name+'.png')});
 async function action(action,extra={}){await page.evaluate(message=>window.__caveSocket.send(JSON.stringify(message)),{type:'action',action,...extra});}
 async function resume(){if(await page.locator('#modal').isVisible())await page.locator('#close-modal').click();if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement);}
 async function face(x,z,pitch=0){const yaw=Math.atan2(-(x-snapshot.me.x),-(z-snapshot.me.z));await page.evaluate(({dx,dy})=>document.dispatchEvent(new MouseEvent('mousemove',{movementX:dx,movementY:dy})),{dx:(snapshot.me.yaw-yaw)/.002,dy:(snapshot.me.pitch-pitch)/.002});await page.waitForTimeout(350);}
 async function travel(x,z){await resume();const start=Date.now();let last=Date.now(),old={x:snapshot.me.x,z:snapshot.me.z};try{while(Math.hypot(x-snapshot.me.x,z-snapshot.me.z)>.45){assert.ok(Date.now()-start<90000,'route timeout toward '+x+','+z);const dx=x-snapshot.me.x,dz=z-snapshot.me.z,d=Math.max(3,Math.hypot(dx,dz));await page.evaluate(input=>window.__caveMovement=input,{x:dx/d,z:dz/d,sprint:false});await page.waitForTimeout(90);if(Date.now()-last>6000){assert.ok(Math.hypot(snapshot.me.x-old.x,snapshot.me.z-old.z)>.2,'route blocked at '+snapshot.me.x+','+snapshot.me.z);old={x:snapshot.me.x,z:snapshot.me.z};last=Date.now();}}}finally{await page.evaluate(()=>window.__caveMovement={x:0,z:0,sprint:false});}await page.waitForTimeout(220);}
 try{
  await until(async()=>{if(server.exitCode!==null)throw Error(log);return fetch(base+'/api/world').then(r=>r.ok).catch(()=>false);},'server start');
  browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
  const context=await browser.newContext({viewport:{width:1440,height:1000}});page=await context.newPage();
  await page.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){window.__caveSocket=this;try{const message=JSON.parse(payload);if(message.type==='input'&&window.__caveMovement)payload=JSON.stringify({...message,...window.__caveMovement});}catch{}return send.call(this,payload);};});
  page.on('pageerror',e=>errors.push(e.message));page.on('response',r=>{if(r.status()>=400)failed.push(r.status()+' '+r.url());});
  page.on('websocket',socket=>{activeSocket=socket;const decoder=createSnapshotDecoder();socket.on('framesent',({payload})=>{const m=JSON.parse(payload.toString());if(m.type==='action')actions.push(m);});socket.on('framereceived',({payload})=>{if(socket!==activeSocket)return;try{const m=JSON.parse(payload.toString()),next=decoder.apply(m);if(next)snapshot=next;if(m.type==='error')errors.push(m.message);}catch(e){errors.push(e.message);}});});
  await page.goto(base,{waitUntil:'networkidle'});await page.locator('#email').fill(email);await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});if(!await page.locator('#login-code').inputValue())await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await until(()=>snapshot?.me,'snapshot');await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);
  assert.equal(snapshot.clubDrinks.length,10,'server drink catalog');
  for(const [x,z] of [[-35,-35],[-35,-192],[-202,-192],[-202,-197]])await travel(x,z);
  await face(-202,-209,.06);await shot('forest-club-entrance');
  assert.equal(await page.locator('#club-light-toggle').isVisible(),true);
  await page.keyboard.press('l');assert.match(await page.locator('#club-light-toggle').innerText(),/reduced/);
  await page.keyboard.press('l');assert.match(await page.locator('#club-light-toggle').innerText(),/on/);
  await travel(-202,-205.5);assert.equal(snapshot.me.inCave,true);assert.equal(snapshot.me.y,0);
  await travel(-202,-218);assert.ok(snapshot.me.y<-6);await face(-202,-235,-.17);await shot('club-stairs');
  await travel(-202,-229);assert.equal(snapshot.me.y,-12);await face(-202,-240,.07);await shot('disco-floor-and-lounge');
  await travel(-212,-232);await face(-216,-232,0);await page.waitForTimeout(200);await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});
  assert.match(await page.locator('#modal-content').innerText(),/Smirnoff Ice/);assert.match(await page.locator('#modal-content').innerText(),/Chocolate shake/);
  const buy=async name=>{const row=page.locator('.shop-row').filter({has:page.getByRole('heading',{name,exact:true})});await row.getByRole('button',{name:/Buy/}).click();};
  await buy('Chocolate shake');await until(()=>snapshot.me.inventory.chocolate_shake===1,'buy chocolate shake');
  await buy('Smirnoff Ice');await until(()=>snapshot.me.inventory.smirnoff_ice===1,'buy Smirnoff Ice');assert.equal(snapshot.me.coins,1985);await shot('club-bar-menu');
  await resume();await page.keyboard.press('i');await page.locator('.storage-bag .storage-slot').filter({hasText:'Chocolate shake'}).click();await page.getByRole('button',{name:'Drink one',exact:true}).click();await until(()=>!snapshot.me.inventory.chocolate_shake,'drink purchased shake');
  await resume();await travel(-213,-242.9);await face(-213,-245,-.3);await page.waitForTimeout(200);await page.keyboard.press('e');await until(()=>snapshot.me.posture==='sitting','sit in lounge chair');await shot('club-seating');await page.keyboard.press('e');await until(()=>snapshot.me.posture==='','stand from chair');
  await travel(-213,-241);await travel(-191,-241);await travel(-191,-237);await face(-187.3,-237,.1);await page.waitForTimeout(200);await page.keyboard.press('e');await page.locator('#club-spin-form').waitFor({state:'visible'});
  assert.equal(await page.locator('.club-odds > div').count(),7);assert.deepEqual(await page.locator('.club-odds > div span').allTextContents().then(rows=>rows.map(row=>parseInt(row))),[7,10,10,10,30,30,3]);const before=snapshot.me.coins;
  await page.locator('#club-stake').fill('37');await page.getByRole('button',{name:'Spin the wheel',exact:true}).click();await until(()=>snapshot.me.clubResult?.stake===37,'spin result');
  let result=snapshot.me.clubResult;assert.ok([5,3,2,1,.5,.1,10].includes(result.multiplier));assert.equal(result.payout,Math.floor(37*Math.round(result.multiplier*10)/10));assert.equal(snapshot.me.coins,before-37+result.payout);await shot('coin-wheel-result');
  await page.waitForTimeout(1300);const balance=snapshot.me.coins,lastID=result.id;
  await page.locator('#club-stake').fill(String(balance));await page.getByRole('button',{name:'Spin the wheel',exact:true}).click();await until(()=>snapshot.me.clubResult?.id!==lastID,'whole wallet stake');assert.equal(snapshot.me.clubResult.stake,balance);assert.equal(snapshot.me.coins,snapshot.me.clubResult.payout);await shot('whole-wallet-spin');
  await resume();await page.keyboard.press('l');assert.match(await page.locator('#club-light-toggle').innerText(),/reduced/);await face(-202,-238,.05);await shot('club-steady-lights');
  await travel(-202,-229);await travel(-202,-197);assert.equal(snapshot.me.inCave,false);assert.equal(snapshot.me.y,0);
  await action('teleport_home');await until(()=>snapshot.me.island===id,'go home');await action('teleport_main');await until(()=>snapshot.me.island===''&&snapshot.me.z>-74,'desert surface return');await travel(120,-58);await face(120,-83,.06);await shot('desert-cave-outside-rocks');
  assert.deepEqual(errors,[],'browser or action errors');assert.deepEqual(failed,[],'failed requests');console.log('PASS: real forest trail and underground club; disco and reduced lights; drinks purchase/consume; chairs; exact coin outcomes and whole-wallet stake; return to surface; desert exterior rocks; no browser errors.');console.log('Screenshots: '+out);
 }catch(e){if(page)await shot('failure').catch(()=>{});console.error({errors,failed,me:snapshot?.me});throw e;}
 finally{if(browser)await browser.close();server.kill();await Promise.race([new Promise(r=>server.once('exit',r)),new Promise(r=>setTimeout(r,3000))]);fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actions,null,2));}
})().catch(e=>{console.error(e);process.exitCode=1;});
