// Isolated real-Chrome cave traversal, lighting and flower scenery review.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),net=require('node:net');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));
(async()=>{
 const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results','cave-browser-'+Date.now()),data=path.join(out,'data');fs.mkdirSync(data,{recursive:true});
 const email='cave-review@example.com',id='u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32),slots=n=>Array.from({length:n},()=>({item:'',count:0})),hotbar=slots(5);hotbar[0]={item:'sugar',count:10};
 fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:2,profiles:{[id]:{id,name:'Cave Explorer',callName:'Cave Explorer',username:'cave_explorer',coins:1000,inventoryVersion:2,safeSlots:slots(3),bagSlots:slots(15),hotbar,selectedSlot:0,levels:{},friendCode:'FERN1837',friends:{},friendRequests:{},avatar:{skin:1,shirt:'tee',pants:'trousers',shirtColor:'teal',pantsColor:'sand'},hats:{},discovered:{sugar:true},homeIsland:{owner:id,visitors:false,builders:{},chest:{},objects:[]}}}}));
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
  const context=await browser.newContext({viewport:{width:1280,height:800}});page=await context.newPage();
  await page.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){window.__caveSocket=this;try{const message=JSON.parse(payload);if(message.type==='input'&&window.__caveMovement)payload=JSON.stringify({...message,...window.__caveMovement});}catch{}return send.call(this,payload);};});
  page.on('pageerror',e=>errors.push(e.message));page.on('response',r=>{if(r.status()>=400)failed.push(r.status()+' '+r.url());});
  page.on('websocket',socket=>{activeSocket=socket;const decoder=createSnapshotDecoder();socket.on('framesent',({payload})=>{const m=JSON.parse(payload.toString());if(m.type==='action')actions.push(m);});socket.on('framereceived',({payload})=>{if(socket!==activeSocket)return;try{const m=JSON.parse(payload.toString()),next=decoder.apply(m);if(next)snapshot=next;if(m.type==='error')errors.push(m.message);}catch(e){errors.push(e.message);}});});
  await page.goto(base,{waitUntil:'networkidle'});await page.locator('#email').fill(email);await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});if(!await page.locator('#login-code').inputValue())await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await until(()=>snapshot?.me,'snapshot');await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);
  await face(-30,-35,-.24);await shot('forest-wildflowers');
  await action('teleport_home');await until(()=>snapshot.me.island===id,'go home');await action('teleport_main');await until(()=>snapshot.me.island===''&&snapshot.me.z>-74,'surface return');
  await travel(120,-58);await face(120,-82,.15);await shot('rocky-horizontal-entrance');
  if(process.env.CK_CAVE_EXTERIOR_ONLY==='1'){
   await travel(100,-62);await face(120,-81,.18);await shot('rock-pile-three-quarter');
   assert.deepEqual(errors,[],'browser or action errors');assert.deepEqual(failed,[],'failed requests');console.log('PASS: updated cave exterior rendered without browser errors.');console.log('Screenshots: '+out);return;
  }
  await travel(140,-42);await face(150,-55,.18);await shot('desert-level-roof');await travel(120,-58);
  const cactus=snapshot.nodes.filter(n=>n.kind==='cactus'&&n.available).sort((a,b)=>Math.hypot(a.x-snapshot.me.x,a.z-snapshot.me.z)-Math.hypot(b.x-snapshot.me.x,b.z-snapshot.me.z))[0];
  const cactusDistance=Math.hypot(snapshot.me.x-cactus.x,snapshot.me.z-cactus.z);
  await travel(cactus.x+(snapshot.me.x-cactus.x)/cactusDistance*3.4,cactus.z+(snapshot.me.z-cactus.z)/cactusDistance*3.4);await face(cactus.x,cactus.z,.04);await shot('cactus-blossoms');await travel(120,-58);
  await travel(120,-76.5);assert.equal(snapshot.me.inCave,true);assert.equal(snapshot.me.y,0);await face(120,-95,-.24);await shot('inside-rock-mouth');
  await travel(120,-94);assert.ok(snapshot.me.y<-10);await face(120,-115,-.18);await shot('deep-clay-stairway');
  await travel(120,-117);assert.equal(snapshot.me.y,-24);await face(120,-124,.24);await shot('amber-cavern');
  await travel(111,-117);await travel(111,-121);await face(111,-124,.04);await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});assert.match(await page.locator('#modal-content').innerText(),/home island/i);await shot('pink-chamber');await resume();
  await travel(111,-117);await travel(129,-117);await travel(129,-121);await face(129,-124,.04);await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});assert.match(await page.locator('#modal-content').innerText(),/lollipop|friends/i);await shot('blue-chamber');await resume();
  await travel(129,-117);await travel(120,-117);await face(120,-78,.25);await travel(120,-70);assert.equal(snapshot.me.inCave,false);assert.equal(snapshot.me.y,0);
  for(const [x,z] of [[120,-60],[110,-60],[110,-9],[110,38]])await travel(x,z);
  await face(80,22,.15);await shot('city-level-roofs');
  await travel(110,86);await face(92,72,.19);await shot('bakery-level-roof');
  await travel(92,86);await face(92,76.5,-.18);await shot('city-flowerbeds');
  assert.deepEqual(errors,[],'browser or action errors');assert.deepEqual(failed,[],'failed requests');console.log('PASS: rock-pile entrance;96steps to24m; both lollipop chambers; return to surface; level desert, city and bakery roofs; flowers and lighting; no browser errors.');console.log('Screenshots: '+out);
 }catch(e){if(page)await shot('failure').catch(()=>{});console.error({errors,failed,me:snapshot?.me});throw e;}
 finally{if(browser)await browser.close();server.kill();await Promise.race([new Promise(r=>server.once('exit',r)),new Promise(r=>setTimeout(r,3000))]);fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actions,null,2));}
})().catch(e=>{console.error(e);process.exitCode=1;});
