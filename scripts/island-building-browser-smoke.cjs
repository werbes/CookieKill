// Real Chrome checks for explicit build mode, aimed edits, packing, gardens,
// personal storage and map shapes. The fixture is isolated from the user's save.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),net=require('node:net');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results',`island-building-${Date.now()}`),data=path.join(out,'data');
  fs.mkdirSync(data,{recursive:true});
  const email='building-controls@example.com',owner='u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32);
  const slots=n=>Array.from({length:n},()=>({item:'',count:0})),bag=slots(20),hotbar=slots(5);
  [['wood',100],['stone',100],['stick',100],['berry',20],['nut',20],['paint_teal',2]].forEach(([item,count],i)=>bag[i]={item,count});hotbar[0]={item:'sugar',count:10};
  const profile={id:owner,name:'Island Builder',callName:'Island Builder',username:'island_builder',coins:500,inventoryVersion:3,bagSlots:bag,hotbar,selectedSlot:0,levels:{},friendCode:'FERN1234',friends:{},friendRequests:{},avatar:{skin:1,shirt:'tee',pants:'trousers',shirtColor:'teal',pantsColor:'sand'},hats:{},discovered:{sugar:true},homeIsland:{owner,visitors:false,builders:{},chest:{wood:20},storage:{personal:{}},objects:[{id:'shared',kind:'chest',x:4,z:18},{id:'personal',kind:'storage_chest',x:-4,z:18},{id:'chair',kind:'chair',x:0,z:14}]}};
  const playerFile=path.join(data,'players',owner+'.json');fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:2,profiles:{[owner]:profile}}));
  const exe=path.join(out,process.platform==='win32'?'server.exe':'server');
  const built=spawnSync(process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go'),['build','-o',exe,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});assert.equal(built.status,0,built.stderr||built.stdout);
  const port=Number(process.env.CK_BUILD_TEST_PORT)||await new Promise((resolve,reject)=>{const probe=net.createServer();probe.on('error',reject);probe.listen(0,'127.0.0.1',()=>{const port=probe.address().port;probe.close(()=>resolve(port));});}),base=`http://127.0.0.1:${port}`;
  const server=spawn(exe,['-dev','-addr',`127.0.0.1:${port}`,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});
  let browser,page,snapshot,serverLog='';const errors=[],actions=[];server.stdout.on('data',b=>serverLog+=b);server.stderr.on('data',b=>serverLog+=b);
  const decoderSource=fs.readFileSync(path.join(root,'web/state-sync.js'),'utf8'),{createSnapshotDecoder}=await import(`data:text/javascript;base64,${Buffer.from(decoderSource).toString('base64')}`);
  async function until(test,message,timeout=15000){const start=Date.now();while(!await test()){assert.ok(Date.now()-start<timeout,message);await new Promise(r=>setTimeout(r,80));}}
  try{
    await until(async()=>{if(server.exitCode!==null)throw Error(serverLog);return fetch(base+'/api/world').then(r=>r.ok).catch(()=>false);},'server start');
    browser=await chromium.launch({headless:true,executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    page=await browser.newPage({viewport:{width:1440,height:1000}});
    await page.addInitScript(()=>{
      const send=WebSocket.prototype.send;WebSocket.prototype.send=function(data){window.__buildSocket=this;const message=JSON.parse(data);if(message.type==='input'&&window.__buildInput)data=JSON.stringify({...message,...window.__buildInput});return send.call(this,data);};
      window.__mapLabels={};const get=HTMLCanvasElement.prototype.getContext;HTMLCanvasElement.prototype.getContext=function(...args){const ctx=get.apply(this,args);if(args[0]==='2d'&&ctx&&!ctx.__tracked){ctx.__tracked=true;const fill=ctx.fillText;ctx.fillText=function(text,...rest){(window.__mapLabels[this.canvas.id]??=[]).push(text);return fill.call(this,text,...rest);};}return ctx;};
    });
    await page.route('**/game.js',async route=>{const response=await route.fetch(),source=await response.text(),anchor='function receiveSnapshot(snapshot){';assert.ok(source.includes(anchor),'read-only build debug anchor');await route.fulfill({response,body:source.replace(anchor,'window.__islandBuildDebug=()=>({yaw:state.yaw,pitch:state.pitch,locked:state.locked,building:state.building});'+anchor)});});
    page.on('pageerror',e=>errors.push(e.message));
    page.on('websocket',socket=>{const decoder=createSnapshotDecoder();socket.on('framesent',({payload})=>{try{const m=JSON.parse(payload.toString());if(m.type==='action')actions.push(m);}catch{}});socket.on('framereceived',({payload})=>{try{const m=JSON.parse(payload.toString()),next=decoder.apply(m);if(next)snapshot=next;if(m.type==='error')errors.push(m.message);}catch(e){errors.push(e.message);}});});
    await page.goto(base,{waitUntil:'networkidle'});await page.locator('#email').fill(email);await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});
    if(!await page.locator('#login-code').inputValue())await page.locator('#login-code').fill((await page.locator('#development-code').innerText()).match(/\b\d{6}\b/)[0]);
    await page.locator('#code-submit').click();await page.locator('#start-button').waitFor({state:'visible'});await until(()=>snapshot?.me,'login snapshot');await page.locator('#start-button').click();
    async function resume(){if(await page.locator('#modal').isVisible())await page.locator('#close-modal').click();if(await page.locator('#play-overlay').isVisible())await page.locator('#play-button').click();if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement&&window.__islandBuildDebug().locked);}
    async function menu(){await resume();await page.keyboard.press('6');await page.locator('#modal').waitFor({state:'visible'});}
    async function face(x,z,y=.7){await resume();const yaw=Math.atan2(-(x-snapshot.me.x),-(z-snapshot.me.z)),pitch=Math.atan2(y-(snapshot.me.y+1.7),Math.hypot(x-snapshot.me.x,z-snapshot.me.z));await page.evaluate(({yaw,pitch})=>{const view=window.__islandBuildDebug(),event=new MouseEvent('mousemove');Object.defineProperty(event,'movementX',{value:(view.yaw-yaw)/.002});Object.defineProperty(event,'movementY',{value:(view.pitch-pitch)/.002});document.dispatchEvent(event);},{yaw,pitch});await page.waitForTimeout(200);}
    async function travel(x,z){await resume();const start=Date.now();while(Math.hypot(x-snapshot.me.x,z-snapshot.me.z)>.4){assert.ok(Date.now()-start<12000,'movement blocked');const dx=x-snapshot.me.x,dz=z-snapshot.me.z,d=Math.hypot(dx,dz);await page.evaluate(input=>window.__buildInput=input,{x:dx/d,z:dz/d,sprint:false});await page.waitForTimeout(100);}await page.evaluate(()=>window.__buildInput={x:0,z:0,sprint:false});await page.waitForTimeout(150);}
    const shot=name=>page.screenshot({path:path.join(out,name+'.png')});
    async function place(){await page.waitForFunction(()=>{const build=window.__islandBuildDebug().building;return build&&Number.isFinite(build.x)&&Number.isFinite(build.z);});await page.mouse.down();await page.mouse.up();}
    const materials=inventory=>Object.fromEntries(['wood','stone','stick','berry','nut','paint_teal'].map(item=>[item,inventory?.[item]||0]));
    let initialMaterials;
    const assertEconomy=coins=>{assert.equal(snapshot.me.coins,coins,'exact coin balance');assert.deepEqual(materials(snapshot.me.inventory),initialMaterials,'coin purchases and packed replacements must preserve materials');};
    async function purchase(category,label,kind,price,x,z){
      await face(x,z,0);await menu();await page.getByRole('button',{name:category,exact:true}).click();
      const card=page.locator('.build-catalog .recipe-card').filter({has:page.getByRole('heading',{name:label,exact:true})});
      assert.match(await card.innerText(),new RegExp(price+' coins'));
      await shot(category.toLowerCase()+'-coin-catalog');
      const before=snapshot.me.coins,count=snapshot.island.objects.filter(o=>o.kind===kind).length;
      await card.getByRole('button',{name:(category==='Garden'?'Plant ':'Place ')+label,exact:true}).click();
      await page.waitForTimeout(200);assertEconomy(before);
      await place();await until(()=>snapshot.island.objects.filter(o=>o.kind===kind).length===count+1,'purchase '+label);
      assertEconomy(before-price);
    }
    await menu();await page.getByRole('button',{name:'Go to my island',exact:true}).click();await until(()=>snapshot.me.island===owner,'arrive home');assert.equal(snapshot.me.buildMode,false);
    await travel(-4,20);await face(-4,18);await page.keyboard.press('e');await page.locator('#modal').waitFor({state:'visible'});assert.match(await page.locator('#modal-title').innerText(),/personal storage/i);
    const sugar=page.locator('.shop-row').filter({has:page.getByRole('heading',{name:'Sugar cookie',exact:true})});await sugar.getByRole('button',{name:'Store one',exact:true}).click();await until(()=>snapshot.island.storage.personal.sugar===1,'store cookie');assert.equal(snapshot.me.inventory.sugar,9);await shot('personal-storage');initialMaterials=materials(snapshot.me.inventory);assertEconomy(500);
    await travel(0,20);await menu();assert.equal(await page.getByRole('button',{name:'Place Wall',exact:true}).isDisabled(),true);assert.doesNotMatch(await page.locator('#modal-content').innerText(),/ON THIS ISLAND/);await page.getByRole('button',{name:'Enter build mode',exact:true}).click();await until(()=>snapshot.me.buildMode,'explicit build mode');
    await face(0,14);await page.keyboard.press('e');await until(()=>snapshot.me.editingBuildId==='chair','aimed E selects chair');await page.keyboard.press('r');await page.keyboard.press('Escape');await until(()=>!snapshot.me.editingBuildId,'cancel edit');assert.equal(snapshot.island.objects.find(o=>o.id==='chair').rotation,0);assert.equal(snapshot.island.objects.find(o=>o.id==='chair').z,14);assertEconomy(500);
    await face(0,14);await page.keyboard.press('e');await until(()=>snapshot.me.editingBuildId==='chair','select chair again');await page.keyboard.press('r');await place();await until(()=>!snapshot.me.editingBuildId&&snapshot.island.objects.find(o=>o.id==='chair').rotation===45,'rotate and place chair');
    let chair=snapshot.island.objects.find(o=>o.id==='chair');await face(chair.x,chair.z);await page.keyboard.press('e');await until(()=>snapshot.me.editingBuildId==='chair','select moved chair');await page.keyboard.press('f');await until(()=>snapshot.me.inventory.build_chair===1&&!snapshot.island.objects.some(o=>o.id==='chair'),'pack with F');await page.keyboard.press('f');assert.equal(snapshot.me.inventory.build_chair,1);assertEconomy(500);
    await menu();await page.locator('.shop-row').filter({has:page.getByRole('heading',{name:'Chair',exact:true})}).getByRole('button',{name:'Place',exact:true}).click();await page.waitForTimeout(200);await place();await until(()=>!snapshot.me.inventory.build_chair&&snapshot.island.objects.some(o=>o.kind==='chair'),'replace packed chair');assertEconomy(500);
    chair=snapshot.island.objects.find(o=>o.kind==='chair');await face(chair.x,chair.z);await page.keyboard.press('e');await until(()=>snapshot.me.editingBuildId===chair.id,'select replacement');await page.keyboard.press('t');await until(()=>!snapshot.island.objects.some(o=>o.id===chair.id),'destroy with T');assertEconomy(500);
    await purchase('Garden','Garden tree','tree',25,0,13);
    await purchase('Garden','Flower bed','flowers',15,6,14);
    await purchase('Decor','Chair','chair',12,-5,15);
    await purchase('Appliances','Oven','oven',120,6,20);
    assertEconomy(328);
    await travel(-4,20);await face(-4,18);await page.keyboard.press('e');await until(()=>snapshot.me.editingBuildId==='personal','select storage for paint');await menu();await page.getByRole('button',{name:'Teal',exact:true}).click();await until(()=>snapshot.island.objects.find(o=>o.id==='personal').color==='teal','paint chest');await page.getByRole('button',{name:'Exit build mode',exact:true}).click();await until(()=>!snapshot.me.buildMode&&!snapshot.me.editingBuildId,'exit releases edit');
    await page.getByRole('button',{name:'World map',exact:true}).click();await page.evaluate(()=>window.__mapLabels['world-map']=[]);await page.waitForTimeout(300);const labels=await page.evaluate(()=>window.__mapLabels['world-map']);assert.ok(labels.length);assert.equal(labels.some(t=>/Garden tree|Flower bed|Personal storage|Shared materials|Chair/.test(t)),false,'island map should draw objects without object-name labels');await shot('island-map-shapes');
    await resume();await face(-4,18);await shot('painted-storage');
    await until(()=>{try{const saved=JSON.parse(fs.readFileSync(playerFile,'utf8')).profile,home=saved.homeIsland;return saved.coins===328&&home.storage?.personal?.sugar===1&&home.objects.some(o=>o.id==='personal'&&o.color==='teal')&&['tree','flowers','chair','oven'].every(kind=>home.objects.filter(o=>o.kind===kind).length===1);}catch{return false;}},'exact purchase balance, objects, storage, and paint persisted',12000);
    const saved=JSON.parse(fs.readFileSync(playerFile,'utf8')).profile;
    assert.deepEqual(materials(saved.inventory),{...initialMaterials,paint_teal:initialMaterials.paint_teal-1},'persistent purchases preserve all materials; only explicit chest painting consumes paint');
    fs.writeFileSync(path.join(out,'economy.json'),JSON.stringify({initialCoins:500,finalCoins:saved.coins,paidPurchases:{tree:25,flowers:15,chair:12,oven:120},materials:materials(saved.inventory),packedReplacementFree:true},null,2));
    assert.deepEqual(errors,[]);assert.ok(actions.some(a=>a.action==='edit_build'));assert.ok(actions.some(a=>a.action==='pack_build'));assert.ok(actions.some(a=>a.action==='destroy_build'));console.log('PASS: explicit build mode, aimed E control, cancel, R rotation, F pack/re-place, T destroy, general personal storage, chest paint, paid Decor/Appliances/Garden with exact balances and unchanged materials, persisted purchases, and map shapes.');console.log('Screenshots: '+out);
  }catch(error){if(page)await page.screenshot({path:path.join(out,'failure.png')}).catch(()=>{});fs.writeFileSync(path.join(out,'failure-snapshot.json'),JSON.stringify({me:snapshot?.me,island:snapshot?.island,errors},null,2));throw error;}
  finally{if(browser)await browser.close();server.kill();await Promise.race([new Promise(r=>server.once('exit',r)),new Promise(r=>setTimeout(r,3000))]);fs.writeFileSync(path.join(out,'server.log'),serverLog);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actions,null,2));}
})().catch(error=>{console.error(error);process.exitCode=1;});
