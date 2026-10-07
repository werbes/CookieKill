// Real Chrome, ordinary server-validated movement inputs, and an isolated Go server.
// npm install --prefix .tools --no-save playwright
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const reopenOnly=process.argv.includes('--reopen-only'),handoverOnly=reopenOnly||process.argv.includes('--handover-only');
  const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results','bakery-browser-'+Date.now()),data=path.join(out,'data');
  fs.mkdirSync(data,{recursive:true});
  const port=Number(process.env.CK_BAKERY_TEST_PORT||8093),base=`http://127.0.0.1:${port}`;
  const ownerEmail='bakery-owner@example.com',otherEmail='bakery-neighbor@example.com';
  const accountID=email=>'u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32),ownerID=accountID(ownerEmail),otherID=accountID(otherEmail);
  const slots=n=>Array.from({length:n},()=>({item:'',count:0}));
  function profile(email,name,code){
    const bagSlots=slots(15),hotbar=slots(5);hotbar[0]={item:'sugar',count:10};
    [['dough',30],['berry',20],['nut',20],['paint_teal',2]].forEach(([item,count],i)=>bagSlots[i]={item,count});
    const returning=handoverOnly&&email===ownerEmail;
    return {id:accountID(email),name,callName:name,username:name.toLowerCase().replaceAll(' ','_'),coins:returning?703:1000,bakery:returning,bakeryLevel:returning?1:0,inventoryVersion:2,safeSlots:slots(3),bagSlots,hotbar,selectedSlot:0,levels:{},friendCode:code,friends:{},friendRequests:{},avatar:{skin:1,shirt:'tee',pants:'trousers',shirtColor:'teal',pantsColor:'sand'},hats:{},discovered:{sugar:true},equipment:{oven:true,mixer:true,display:true},paint:returning?{walls:'teal'}:{}};
  }
  fs.writeFileSync(path.join(data,'players.json'),JSON.stringify({version:2,profiles:{[ownerID]:profile(ownerEmail,'Bakery Owner','FERN3821'),[otherID]:profile(otherEmail,'Bakery Neighbor','MINT9146')}}));
  const exe=path.join(out,process.platform==='win32'?'server.exe':'server'),go=process.env.CK_GO||(process.platform==='win32'?'C:/Program Files/Go/bin/go.exe':'go');
  const build=spawnSync(go,['build','-o',exe,'.'],{cwd:root,windowsHide:true,encoding:'utf8'});assert.equal(build.status,0,build.stderr||build.stdout);
  assert.equal(await fetch(base+'/api/world').then(()=>true).catch(()=>false),false,'Choose an unused CK_BAKERY_TEST_PORT');
  let log='',browser,failurePage;const errors=[],actions=[];
  const server=spawn(exe,['-dev','-addr',`127.0.0.1:${port}`,'-data',data],{cwd:root,windowsHide:true,stdio:['ignore','pipe','pipe']});
  server.stdout.on('data',x=>log+=x);server.stderr.on('data',x=>log+=x);
  const decoderSource=fs.readFileSync(path.join(root,'web/state-sync.js'),'utf8'),{createSnapshotDecoder}=await import(`data:text/javascript;base64,${Buffer.from(decoderSource).toString('base64')}`);
  async function until(check,message,timeout=15000){const started=Date.now();while(!await check()){assert.ok(Date.now()-started<timeout,message);await new Promise(resolve=>setTimeout(resolve,90));}}
  try{
    await until(async()=>{if(server.exitCode!==null)throw new Error(log);return fetch(base+'/api/world').then(r=>r.ok).catch(()=>false);},'server startup',20000);
    browser=await chromium.launch({executablePath:process.env.CK_CHROME||(process.platform==='win32'?'C:/Program Files/Google/Chrome/Application/chrome.exe':undefined),headless:true,args:['--use-angle=swiftshader','--enable-unsafe-swiftshader','--enable-webgl']});
    async function login(email,context){
      const returningSession=!!context;
      context=context||await browser.newContext({viewport:{width:960,height:720}});
      const page=await context.newPage(),client={page,context,snapshot:null,yaw:0,pitch:0};failurePage=page;
      // Only directional movement intent is overridden, exactly as in the island
      // smoke tour. Go still owns positions, speed, walls, and all other physics.
      await page.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){try{const message=JSON.parse(payload);if(message.type==='input'&&window.__bakeryMovement)payload=JSON.stringify({...message,...window.__bakeryMovement});}catch{}return send.call(this,payload);};});
      // Software WebGL is expensive with two accounts. Only skip presentation
      // frames in this test response; UI, network, input, and simulation are intact.
      await page.route('**/game.js',async route=>{const response=await route.fetch(),source=await response.text();await route.fulfill({response,body:source.replace('renderer.render(scene,camera);','window.__bakeryRenderFrame=(window.__bakeryRenderFrame||0)+1;if(window.__bakeryRenderFrame%3===0)renderer.render(scene,camera);')});});
      page.on('pageerror',e=>errors.push(e.message));
      page.on('websocket',socket=>{const decode=createSnapshotDecoder();socket.on('framereceived',({payload})=>{try{const message=JSON.parse(payload.toString()),update=decode.apply(message);if(update)client.snapshot=update;if(message.type==='error')client.lastError=message.message;}catch(e){errors.push(e.message);}});socket.on('framesent',({payload})=>{try{const message=JSON.parse(payload.toString());if(message.type==='action')actions.push({...message,email});}catch{}});});
      await page.goto(base,{waitUntil:'networkidle'});
      if(!returningSession){
        await page.locator('#email').fill(email);await page.locator('#email-submit').click();await page.locator('#code-stage').waitFor({state:'visible'});
        if(!await page.locator('#login-code').inputValue()){const code=(await page.locator('#development-code').innerText()).match(/\b\d{6}\b/);assert.ok(code);await page.locator('#login-code').fill(code[0]);}
        await page.locator('#code-submit').click();
      }
      await page.locator('#start-button').waitFor({state:'visible'});await until(()=>client.snapshot?.me,'snapshot after login');await page.locator('#start-button').click();await page.waitForFunction(()=>!!document.pointerLockElement);return client;
    }
    async function resume(client){const page=client.page;await page.bringToFront();failurePage=page;if(await page.locator('#modal').isVisible())await page.locator('#close-modal').click();if(await page.locator('#play-overlay').isVisible())await page.locator('#play-button').click();if(await page.locator('#capture-prompt').isVisible())await page.locator('#capture-prompt').click();await page.waitForFunction(()=>!!document.pointerLockElement);}
    async function face(client,x,z,pitch=0){const yaw=Math.atan2(-(x-client.snapshot.me.x),-(z-client.snapshot.me.z));await client.page.evaluate(({dx,dy})=>document.dispatchEvent(new MouseEvent('mousemove',{movementX:dx,movementY:dy})),{dx:(client.snapshot.me.yaw-yaw)/.002,dy:(client.snapshot.me.pitch-pitch)/.002});await client.page.waitForTimeout(160);}
    async function travel(client,x,z){
      await resume(client);const page=client.page,started=Date.now();let last=Date.now(),old={x:client.snapshot.me.x,z:client.snapshot.me.z};
      try{while(Math.hypot(x-client.snapshot.me.x,z-client.snapshot.me.z)>.65){
        assert.ok(Date.now()-started<180000,`route timed out at ${client.snapshot.me.x},${client.snapshot.me.z} toward ${x},${z}`);
        const dx=x-client.snapshot.me.x,dz=z-client.snapshot.me.z,d=Math.hypot(dx,dz),approach=Math.max(12,d);await page.evaluate(input=>window.__bakeryMovement=input,{x:dx/approach,z:dz/approach,sprint:false});await page.waitForTimeout(90);
        if(Date.now()-last>4500){assert.ok(Math.hypot(client.snapshot.me.x-old.x,client.snapshot.me.z-old.z)>.15,`route blocked at ${client.snapshot.me.x},${client.snapshot.me.z} toward ${x},${z}`);old={x:client.snapshot.me.x,z:client.snapshot.me.z};last=Date.now();}
      }}finally{await page.evaluate(()=>window.__bakeryMovement={x:0,z:0,sprint:false});}await page.waitForTimeout(180);
    }
    async function route(client,points){for(const [x,z] of points)await travel(client,x,z);}
    async function open(client,expected){await resume(client);await client.page.waitForTimeout(200);await client.page.keyboard.press('e');await client.page.locator('#modal').waitFor({state:'visible'});assert.match(await client.page.locator('#modal-content').innerText(),expected);}
    const shot=(client,name)=>client.page.screenshot({path:path.join(out,name+'.png')});
    const contentActions=client=>client.page.locator('#modal-content [data-action]').evaluateAll(buttons=>[...new Set(buttons.map(button=>button.dataset.action))].sort());
    const owner=await login(ownerEmail);assert.equal(owner.snapshot.me.id,ownerID);
    if(reopenOnly){
      const saved=structuredClone(owner.snapshot.me);assert.equal(saved.bakery,true);assert.equal(saved.bakeryPlot,'');
      await route(owner,[[-35,-35],[-35,-9],[8,-9],[8,84],[58,84],[58,78],[58,74],[58,72]]);await face(owner,58,68);await open(owner,/Claim this plot for free/);await shot(owner,'returning-free-claim');
      await owner.page.getByRole('button',{name:'Claim plot · Free',exact:true}).click();await until(()=>owner.snapshot.me.bakeryPlot==='bakery_plot_1','free claim');
      assert.equal(owner.snapshot.me.coins,saved.coins);assert.equal(owner.snapshot.me.bakeryLevel,saved.bakeryLevel);assert.deepEqual(owner.snapshot.me.equipment,saved.equipment);assert.deepEqual(owner.snapshot.me.paint,saved.paint);assert.equal(owner.snapshot.nodes.find(n=>n.id==='bakery_plot_1').owner,ownerID);await shot(owner,'saved-bakery-restored');
      await route(owner,[[58,71.5],[55.4,70.9]]);await face(owner,55.4,69.5);await open(owner,/Every recipe is recorded/);const before=owner.snapshot.me.inventory.sugar;await owner.page.locator('.recipe-card').first().getByRole('button',{name:'Bake one',exact:true}).click();await until(()=>owner.snapshot.me.inventory.sugar===before+1,'reopened oven works');await shot(owner,'reopened-oven');
      assert.deepEqual(errors,[]);console.log('PASS: saved bakery free claim preserves coins/upgrades/equipment/paint and reopened oven bakes through real UI');console.log('Screenshots: '+out);return;
    }
    await route(owner,[[-35,-35],[-35,-9],[8,-9],[8,84],[24,84],[24,78],[24,74],[24,72]]);
    await face(owner,24,68);
    if(handoverOnly){await open(owner,/Claim this plot for free/);await owner.page.getByRole('button',{name:'Claim plot · Free',exact:true}).click();await until(()=>owner.snapshot.me.bakeryPlot==='bakery_plot_0','opening saved bakery fixture');assert.equal(owner.snapshot.me.coins,703);}
    else{
    await open(owner,/Buy your first bakery/);await shot(owner,'claim-first-bakery');
    await owner.page.getByRole('button',{name:'Buy bakery · 250',exact:true}).click();await until(()=>owner.snapshot.me.bakeryPlot==='bakery_plot_0','first bakery purchase');assert.equal(owner.snapshot.me.coins,750);
    await until(async()=>await owner.page.getByRole('button',{name:'Upgrade · 50',exact:true}).isVisible(),'management refreshed');
    await owner.page.getByRole('button',{name:'Upgrade · 50',exact:true}).click();await until(()=>owner.snapshot.me.bakeryLevel===1,'upgrade saved');assert.equal(owner.snapshot.me.coins,700);
    const walls=owner.page.locator('.shop-row').filter({has:owner.page.getByRole('heading',{name:'Walls',exact:true})});await walls.getByRole('button',{name:'Teal',exact:true}).click();await until(()=>owner.snapshot.me.paint.walls==='teal','paint saved');assert.deepEqual(await contentActions(owner),['paint','upgrade']);await shot(owner,'manage-bakery');
    await owner.page.locator('#bakery-name').fill('Moonflower Bakery');
    await owner.page.locator('#bakery-name-form').getByRole('button',{name:'Save name',exact:true}).click();
    await until(()=>owner.snapshot.me.bakeryName==='Moonflower Bakery','bakery name saved');
    await until(()=>owner.snapshot.nodes.find(node=>node.id==='bakery_plot_0')?.label==='Moonflower Bakery','bakery sign reflects saved name');
    await shot(owner,'renamed-bakery');
    console.log('PASS: first bakery purchase, upgrade, paint, and rename via UI');
    await route(owner,[[24,71.5],[21.4,70.9]]);await face(owner,21.4,69.5);await open(owner,/Every recipe is recorded/);
    assert.deepEqual(await contentActions(owner),['craft']);assert.equal(await owner.page.getByRole('button',{name:'Build campfire',exact:true}).count(),0);assert.equal(await owner.page.locator('.recipe-card').count(),8);
    const beforeSugar=owner.snapshot.me.inventory.sugar;await owner.page.locator('.recipe-card').first().getByRole('button',{name:'Bake one',exact:true}).click();await until(()=>owner.snapshot.me.inventory.sugar===beforeSugar+1,'oven bake: '+owner.lastError);await shot(owner,'oven-recipes-only');
    await route(owner,[[24,71.1],[26.5,70.9]]);await face(owner,26.5,69.5);await open(owner,/Mix fresh dough/);assert.deepEqual(await contentActions(owner),['make_dough']);
    const beforeDough=owner.snapshot.me.inventory.dough;await owner.page.getByRole('button',{name:'Mix dough',exact:true}).click();await until(()=>owner.snapshot.me.inventory.dough===beforeDough+3,'mixer dough: '+owner.lastError);await shot(owner,'mixer-dough-only');
    await route(owner,[[24,71.6],[26.4,72.7]]);await face(owner,26.4,74);await open(owner,/Sell the cookies and cakes/);assert.deepEqual(await contentActions(owner),['trade']);
    const beforeCoins=owner.snapshot.me.coins;await owner.page.locator('.shop-row').first().getByRole('button',{name:'Sell one · 3 coins',exact:true}).click();await until(()=>owner.snapshot.me.coins===beforeCoins+3,'display sale: '+owner.lastError);await shot(owner,'display-selling-only');
    console.log('PASS: oven recipes only, mixer dough only, display sales only');
    }
    const saved=structuredClone(owner.snapshot.me);await resume(owner);await route(owner,[[24,72],[24,74]]);
    await owner.page.setViewportSize({width:480,height:320});
    const neighbor=await login(otherEmail);await route(neighbor,[[-35,-35],[-35,-9],[8,-9],[8,84],[24,84],[24,78],[24,74],[24,72]]);await open(neighbor,/belongs to Bakery Owner/);await shot(neighbor,'occupied-plot');
    const context=owner.context;await owner.page.close();await until(()=>neighbor.snapshot.nodes.find(n=>n.id==='bakery_plot_0')?.owner==='','plot released on socket disconnect');
    await until(async()=>await neighbor.page.getByRole('button',{name:'Buy bakery · 250',exact:true}).isVisible(),'released plot panel refreshed');await neighbor.page.getByRole('button',{name:'Buy bakery · 250',exact:true}).click();await until(()=>neighbor.snapshot.me.bakeryPlot==='bakery_plot_0','second buyer claim');assert.equal(neighbor.snapshot.me.coins,750);await shot(neighbor,'neighbor-claims-released-plot');
    console.log('PASS: immediate socket release and second-account purchase');
    const returning=await login(ownerEmail,context);assert.equal(returning.snapshot.me.bakery,true);assert.equal(returning.snapshot.me.bakeryPlot,'');
    await route(returning,[[24,78],[24,84],[58,84],[58,78],[58,74],[58,72]]);await face(returning,58,68);await open(returning,/Claim this plot for free/);await shot(returning,'returning-free-claim');
    await returning.page.getByRole('button',{name:'Claim plot · Free',exact:true}).click();await until(()=>returning.snapshot.me.bakeryPlot==='bakery_plot_1','free claim');
    assert.equal(returning.snapshot.me.coins,saved.coins);assert.equal(returning.snapshot.me.bakeryLevel,saved.bakeryLevel);assert.deepEqual(returning.snapshot.me.equipment,saved.equipment);assert.deepEqual(returning.snapshot.me.paint,saved.paint);assert.equal(returning.snapshot.me.bakeryName,saved.bakeryName);
    assert.equal(returning.snapshot.nodes.find(n=>n.id==='bakery_plot_0').owner,otherID);assert.equal(returning.snapshot.nodes.find(n=>n.id==='bakery_plot_1').owner,ownerID);await shot(returning,'saved-bakery-restored');
    await route(returning,[[58,71.5],[55.4,70.9]]);await face(returning,55.4,69.5);await open(returning,/Every recipe is recorded/);const afterSugar=returning.snapshot.me.inventory.sugar;await returning.page.locator('.recipe-card').first().getByRole('button',{name:'Bake one',exact:true}).click();await until(()=>returning.snapshot.me.inventory.sugar===afterSugar+1,'reopened oven works');
    assert.deepEqual(errors,[]);console.log(handoverOnly?'PASS: real movement, immediate socket release, second buyer, free saved-bakery claim preserving upgrades/equipment/paint, and reopened oven':'PASS: real movement, first claim, upgrade/paint, separate oven/mixer/display menus and actions, socket release, second buyer, free saved-bakery claim');console.log('Screenshots: '+out);
  }catch(error){if(failurePage&&!failurePage.isClosed())await failurePage.screenshot({path:path.join(out,'failure.png')}).catch(()=>{});throw error;}
  finally{if(browser)await browser.close();server.kill();fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actions,null,2));}
})().catch(error=>{console.error(error);process.exitCode=1;});
