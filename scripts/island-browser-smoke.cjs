// Real Chromium + isolated Go server. Requires the same local Playwright install as browser-smoke.cjs.
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results',`island-browser-${Date.now()}`),data=path.join(out,'data');
  fs.mkdirSync(data,{recursive:true});
  const port=Number(process.env.CK_ISLAND_TEST_PORT||8091),base=`http://127.0.0.1:${port}`;
  const ownerEmail='island-builder@example.com',friendEmail='island-friend@example.com';
  const id=email=>'u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32);
  const ownerID=id(ownerEmail),friendID=id(friendEmail);
  const emptySlots=n=>Array.from({length:n},()=>({item:'',count:0}));
  function profile(email,callName,friendCode,ownsIsland){
    const pid=id(email),bagSlots=emptySlots(15),hotbar=emptySlots(5);
    [['wood',200],['stick',100],['stone',200],['berry',50],['nut',50]].forEach(([item,count],i)=>{bagSlots[i]={item,count};});hotbar[0]={item:'sugar',count:10};
    return {id:pid,name:callName,callName,username:callName.replaceAll(' ','_').toLowerCase(),coins:500,inventoryVersion:2,safeSlots:emptySlots(3),bagSlots,hotbar,selectedSlot:0,levels:{},friendCode,friends:{},friendRequests:{},avatar:{skin:1,shirt:'tee',pants:'trousers',shirtColor:'teal',pantsColor:'sand'},hats:{},discovered:{},...(ownsIsland?{homeIsland:{owner:pid,visitors:false,builders:{},chest:{wood:30},objects:[{id:'welcome_chest',kind:'chest',x:4,y:0,z:18,rotation:0}]}}:{})};
  }
  fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:2,profiles:{[ownerID]:profile(ownerEmail,'Island Builder','FERN1234',true),[friendID]:profile(friendEmail,'Island Friend','MINT2468',false)}}));
  const go=process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go'),exe=path.join(out,process.platform==='win32'?'server.exe':'server');
  const built=spawnSync(go,['build','-o',exe,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});assert.equal(built.status,0,built.stderr||built.stdout);
  assert.equal(await fetch(base+'/api/world').then(()=>true).catch(()=>false),false,'Choose an unused CK_ISLAND_TEST_PORT');
  let serverLog='',browser;const actionLog=[];const server=spawn(exe,['-dev','-addr',`127.0.0.1:${port}`,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});
  server.stdout.on('data',chunk=>{serverLog+=chunk;});server.stderr.on('data',chunk=>{serverLog+=chunk;});
  const source=fs.readFileSync(path.join(root,'web/state-sync.js'),'utf8'),{createSnapshotDecoder}=await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  const errors=[];let pageForFailure;
  async function until(test,message,timeout=12000){const start=Date.now();while(!await test()){assert.ok(Date.now()-start<timeout,message);await new Promise(resolve=>setTimeout(resolve,80));}}
  try{
    await until(async()=>{if(server.exitCode!==null)throw new Error(serverLog);try{return(await fetch(base+'/api/world')).ok;}catch{return false;}},'server start',20000);
    browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    async function login(email){
      const context=await browser.newContext({viewport:{width:1440,height:1000}}),page=await context.newPage(),client={page,context,snapshot:null};pageForFailure=page;
      await page.addInitScript(()=>{const original=WebSocket.prototype.send;WebSocket.prototype.send=function(data){window.__islandSocket=this;try{const message=JSON.parse(data);if(message.type==='input'&&window.__islandInput)data=JSON.stringify({...message,...window.__islandInput});}catch{}return original.call(this,data);};});
      page.on('pageerror',error=>errors.push(error.message));
      page.on('websocket',socket=>{const decoder=createSnapshotDecoder();socket.on('framesent',({payload})=>{try{const message=JSON.parse(payload.toString());if(message.type==='action')actionLog.push(message);}catch{}});socket.on('framereceived',({payload})=>{try{const message=JSON.parse(payload.toString()),snapshot=decoder.apply(message);if(snapshot)client.snapshot=snapshot;if(message.type==='error')client.lastError=message.message;}catch(error){errors.push(error.message);}});});
      await page.goto(base,{waitUntil:'networkidle'});await page.locator('#email').fill(email);await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});
      if(!await page.locator('#login-code').inputValue()){const code=(await page.locator('#development-code').innerText()).match(/\b\d{6}\b/);assert.ok(code);await page.locator('#login-code').fill(code[0]);}
      await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await until(()=>client.snapshot?.me,'snapshot after login');await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);return client;
    }
    async function resume(page){if(await page.locator('#modal').isVisible())await page.locator('#close-modal').click();if(await page.locator('#play-overlay').isVisible())await page.locator('#play-button').click();if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement);}
    async function menu(page){await resume(page);await page.keyboard.press('6');await page.locator('#modal').waitFor({state:'visible'});}
    const screenshot=(page,name)=>page.screenshot({path:path.join(out,name+'.png')});
    const owner=await login(ownerEmail),page=owner.page;assert.equal(owner.snapshot.me.id,ownerID);assert.equal(await page.locator('#hotbar .hotbar-slot').count(),6);
    await menu(page);await page.getByRole('button',{name:'Go to my island',exact:true}).click();await until(()=>owner.snapshot.me.island===ownerID,'teleport to home');await resume(page);await screenshot(page,'home-island');
    await menu(page);assert.match(await page.locator('#modal-content').innerText(),/Building.*Decor.*Appliances/s);await screenshot(page,'build-catalog');
    await page.getByRole('button',{name:'Place Wall',exact:true}).click();await page.waitForFunction(()=>!!document.pointerLockElement);await page.waitForTimeout(300);await page.keyboard.press('r');await page.waitForTimeout(250);await screenshot(page,'wall-preview');await page.mouse.click(720,500);
    await until(()=>owner.snapshot.island.objects.some(object=>object.kind==='wall'),'place wall: '+owner.lastError);let wall=owner.snapshot.island.objects.find(object=>object.kind==='wall');assert.equal(wall.rotation,45);assert.equal(owner.snapshot.me.inventory.wood,196);
    await page.keyboard.press('Escape');await resume(page);await menu(page);
    const inspectWall=async()=>page.locator('.shop-row').filter({has:page.getByRole('heading',{name:'Wall',exact:true})}).getByRole('button',{name:'Inspect',exact:true}).click();
    await inspectWall();await page.getByRole('button',{name:'Rotate 45°',exact:true}).click();await until(()=>owner.snapshot.island.objects.find(object=>object.id===wall.id)?.rotation===90,'rotate placed wall');await screenshot(page,'object-controls');
    await page.getByRole('button',{name:'Move',exact:true}).click();await page.waitForFunction(()=>!!document.pointerLockElement);await page.evaluate(()=>document.dispatchEvent(new MouseEvent('mousemove',{movementX:350,movementY:0})));await page.waitForTimeout(180);await page.mouse.click(720,500);
    await until(()=>owner.snapshot.island.objects.find(object=>object.id===wall.id)?.x!==wall.x,'move wall: '+owner.lastError);await menu(page);await inspectWall();await page.getByRole('button',{name:'Destroy…',exact:true}).click();await page.getByRole('button',{name:'Yes, destroy',exact:true}).click();await until(()=>!owner.snapshot.island.objects.some(object=>object.id===wall.id),'destroy wall');assert.equal(owner.snapshot.me.inventory.wood,198);assert.equal(owner.snapshot.me.inventory.stone,199);
    await page.getByRole('button',{name:'Home island',exact:true}).click();await page.getByRole('button',{name:'Island material chest',exact:true}).click();const woodRow=page.locator('.shop-row').filter({has:page.getByRole('heading',{name:'Wood',exact:true})});await woodRow.getByRole('button',{name:'Give 20',exact:true}).click();await until(()=>owner.snapshot.island.chest.wood===50,'donate to chest');await woodRow.getByRole('button',{name:'Take 20',exact:true}).click();await until(()=>owner.snapshot.island.chest.wood===30,'withdraw from chest');await screenshot(page,'island-chest');
    await page.getByRole('button',{name:'Friends',exact:true}).click();await page.getByRole('button',{name:'Show my code publicly',exact:true}).click();await until(()=>owner.snapshot.me.publicFriendCode,'publish code');await page.locator('#friend-code-input').fill('MINT2468');await page.getByRole('button',{name:'Send request',exact:true}).click();
    const friend=await login(friendEmail);await friend.page.keyboard.press('Escape');await friend.page.locator('#friends-button').click();await until(()=>friend.snapshot.friends.some(entry=>entry.id===ownerID&&entry.request),'incoming friend request');assert.match(await friend.page.locator('#modal-content').innerText(),/Island Builder/);await screenshot(friend.page,'friend-request');await friend.page.getByRole('button',{name:'Accept',exact:true}).click();await until(()=>friend.snapshot.me.friends[ownerID]&&owner.snapshot.me.friends[friendID],'accept friendship');
    await page.bringToFront();pageForFailure=page;await page.getByRole('button',{name:'Allow visitors',exact:true}).click();await until(()=>owner.snapshot.me.homeIsland.visitors,'allow visitors');await page.getByRole('button',{name:'Allow this friend to build',exact:true}).click();await until(()=>owner.snapshot.me.homeIsland.builders[friendID],'explicit builder permission');await screenshot(page,'friends-permissions');
    await page.getByRole('button',{name:'Return to main island',exact:true}).click();await until(()=>owner.snapshot.me.island==='','return to main');await resume(page);
    async function face(x,z){const yaw=Math.atan2(-(x-owner.snapshot.me.x),-(z-owner.snapshot.me.z));await page.evaluate(({dx,dy})=>document.dispatchEvent(new MouseEvent('mousemove',{movementX:dx,movementY:dy})),{dx:(owner.snapshot.me.yaw-yaw)/.002,dy:owner.snapshot.me.pitch/.002});await page.waitForTimeout(160);}
    async function travel(x,z){const started=Date.now();while(Math.hypot(x-owner.snapshot.me.x,z-owner.snapshot.me.z)>.65){assert.ok(Date.now()-started<20000,`route blocked at ${owner.snapshot.me.x},${owner.snapshot.me.z}`);const dx=x-owner.snapshot.me.x,dz=z-owner.snapshot.me.z,d=Math.hypot(dx,dz);await page.evaluate(input=>window.__islandInput=input,{x:dx/d,z:dz/d,sprint:false});await page.waitForTimeout(100);}await page.evaluate(()=>window.__islandInput={x:0,z:0,sprint:false});await page.waitForTimeout(160);}
    await face(120,-115);await screenshot(page,'cave-entrance');await travel(120,-115);assert.ok(owner.snapshot.me.y<-3,'cave descends below ground');await screenshot(page,'cave-fork');await travel(111,-121);await face(111,-124);await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});assert.match(await page.locator('#modal-content').innerText(),/Would you like to teleport/);await screenshot(page,'pink-lollipop');await resume(page);await travel(120,-117);await travel(129,-121);await face(129,-124);await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});await page.getByRole('button',{name:'Eat the blue lollipop',exact:true}).click();await until(()=>owner.snapshot.me.friendsTeleport,'blue lollipop unlock');await page.getByRole('button',{name:'Visit island',exact:true}).waitFor();await screenshot(page,'blue-lollipop');
    assert.deepEqual(errors,[]);console.log('PASS: real home teleport, sixth hotbar, preview rotation, place/move/destroy refunds, chest, friend request/privacy/permissions, cave lollipops');console.log('Screenshots: '+out);
  }catch(error){if(pageForFailure)await pageForFailure.screenshot({path:path.join(out,'failure.png')}).catch(()=>{});throw error;}
  finally{if(browser)await browser.close();server.kill();await Promise.race([new Promise(resolve=>server.once('exit',resolve)),new Promise(resolve=>setTimeout(resolve,3000))]);fs.writeFileSync(path.join(out,'server.log'),serverLog);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actionLog,null,2));}
})().catch(error=>{console.error(error);process.exitCode=1;});
