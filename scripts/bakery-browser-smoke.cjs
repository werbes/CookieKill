// Real Chrome, ordinary server-validated movement inputs, and an isolated Go server.
// npm install --prefix .tools --no-save playwright
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const {spawn,spawnSync}=require('node:child_process');
const {chromium}=require(require.resolve('playwright',{paths:[path.resolve(__dirname,'../.tools')]}));

(async()=>{
  const queueVisual=process.argv.includes('--queue-visual-only'),queuesOnly=queueVisual||process.argv.includes('--queues-only');
  const buildingOnly=process.argv.includes('--building-only'),productionOnly=process.argv.includes('--production-only');
  const staffOnly=process.argv.includes('--staff-only'),reopenOnly=process.argv.includes('--reopen-only'),handoverOnly=queuesOnly||productionOnly||buildingOnly||staffOnly||reopenOnly||process.argv.includes('--handover-only');
  const root=path.resolve(__dirname,'..'),out=path.join(root,'test-results','bakery-browser-'+Date.now()),data=path.join(out,'data');
  fs.mkdirSync(data,{recursive:true});
  const port=Number(process.env.CK_BAKERY_TEST_PORT||8093),base=`http://127.0.0.1:${port}`;
  const ownerEmail='bakery-owner@example.com',otherEmail='bakery-neighbor@example.com';
  const accountID=email=>'u_'+crypto.createHash('sha256').update(email).digest('hex').slice(0,32),ownerID=accountID(ownerEmail),otherID=accountID(otherEmail);
  const slots=n=>Array.from({length:n},()=>({item:'',count:0}));
  function profile(email,name,code){
    const bagSlots=slots(15),hotbar=slots(5);hotbar[0]={item:'sugar',count:10};
    [['dough',30],['berry',20],['nut',20],['paint_teal',2],...(!productionOnly?[['baking_sugar',5],['butter',5],['flour',5],['egg',5]]:[])].filter(([item])=>!productionOnly||item!=='dough').forEach(([item,count],i)=>bagSlots[i]={item,count});
    const returning=handoverOnly&&email===ownerEmail;
    return {production:queueVisual&&returning?{bakery_mixer:{job:{item:'dough',amount:40,remaining:40,total:40},queue:[{item:'prepared_berry_cookie',amount:5,remaining:5,total:5},{item:'prepared_berry_cookie',amount:1,remaining:1,total:1}],output:{}},bakery_cooling_table:{output:{sugar:7}}}:{},id:accountID(email),name,callName:name,username:name.toLowerCase().replaceAll(' ','_'),coins:queuesOnly||buildingOnly||productionOnly||staffOnly?2000:returning?703:email===ownerEmail?1050:1000,bakery:returning,bakeryLevel:returning?1:0,inventoryVersion:2,safeSlots:slots(3),bagSlots,hotbar,selectedSlot:0,levels:{},friendCode:code,friends:{},friendRequests:{},avatar:{skin:1,shirt:'tee',pants:'trousers',shirtColor:'teal',pantsColor:'sand'},hats:{},discovered:{sugar:true},equipment:{oven:true,mixer:true,display:true},paint:returning?{walls:'teal'}:{}};
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
      await page.addInitScript(()=>{const send=WebSocket.prototype.send;WebSocket.prototype.send=function(payload){try{const message=JSON.parse(payload);window.__bakerySocket=this;if(message.type==='input'&&window.__bakeryMovement)payload=JSON.stringify({...message,...window.__bakeryMovement});}catch{}return send.call(this,payload);};});
      // Software WebGL is expensive with two accounts. Only skip presentation
      // frames in this test response; UI, network, input, and simulation are intact.
      await page.route('**/game.js',async route=>{const response=await route.fetch(),source=await response.text();await route.fulfill({response,body:source.replace('function receiveSnapshot(snapshot){','window.__bakeryDebug=()=>({locked:state.locked,yaw:state.yaw,pitch:state.pitch,modal:state.modal,building:state.building,pending:state.buildOperationPending,editing:state.snapshot?.me.editingBakeryFurnitureId});function receiveSnapshot(snapshot){').replace('renderer.render(scene,camera);','window.__bakeryRenderFrame=(window.__bakeryRenderFrame||0)+1;if(window.__bakeryRenderFrame%3===0)renderer.render(scene,camera);')});});
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
    async function face(client,x,z,pitch=0){await client.page.waitForFunction(()=>!!document.pointerLockElement&&window.__bakeryDebug?.().locked);const yaw=Math.atan2(-(x-client.snapshot.me.x),-(z-client.snapshot.me.z));await client.page.evaluate(({yaw,pitch})=>{const view=window.__bakeryDebug();const event=new MouseEvent('mousemove');Object.defineProperty(event,'movementX',{value:(view.yaw-yaw)/.002});Object.defineProperty(event,'movementY',{value:(view.pitch-pitch)/.002});document.dispatchEvent(event);},{yaw,pitch});await client.page.waitForTimeout(180);}
    async function travel(client,x,z){
      await resume(client);const page=client.page,started=Date.now();let last=Date.now(),old={x:client.snapshot.me.x,z:client.snapshot.me.z};
      try{while(Math.hypot(x-client.snapshot.me.x,z-client.snapshot.me.z)>.65){
        assert.ok(Date.now()-started<180000,`route timed out at ${client.snapshot.me.x},${client.snapshot.me.z} toward ${x},${z}`);
        const dx=x-client.snapshot.me.x,dz=z-client.snapshot.me.z,d=Math.hypot(dx,dz),approach=Math.max(4,d);await page.evaluate(input=>window.__bakeryMovement=input,{x:dx/approach,z:dz/approach,sprint:false});await page.waitForTimeout(90);
        if(Date.now()-last>4500){assert.ok(Math.hypot(client.snapshot.me.x-old.x,client.snapshot.me.z-old.z)>.15,`route blocked at ${client.snapshot.me.x},${client.snapshot.me.z} toward ${x},${z}`);old={x:client.snapshot.me.x,z:client.snapshot.me.z};last=Date.now();}
      }}finally{await page.evaluate(()=>window.__bakeryMovement={x:0,z:0,sprint:false});}await page.waitForTimeout(180);
    }
    async function route(client,points){for(const [x,z] of points)await travel(client,x,z);}
    async function open(client,expected){await resume(client);await client.page.waitForTimeout(200);await client.page.keyboard.press('e');await client.page.locator('#modal').waitFor({state:'visible'});assert.match(await client.page.locator('#modal-content').innerText(),expected);}
    function plotOf(client){return client.snapshot.nodes.find(n=>n.id===client.snapshot.me.bakeryPlot);}
    async function management(client){const p=plotOf(client);await route(client,[[p.x,client.snapshot.me.z],[p.x,p.z+1.2]]);await face(client,p.x,p.z);await open(client,/YOUR BAKERY TEAM/);}
    async function station(client,kind,expected){
      const p=plotOf(client),n=client.snapshot.nodes.find(n=>n.id===p.id+'_'+kind);assert.ok(n,'missing station '+kind);
      const kitchen=kind!=='display',offset=kind==='display'?2.1:1.8;
      await route(client,[[p.x,client.snapshot.me.z],[p.x,p.z+(kitchen?-3.5:5)],[n.x,n.z+offset]]);await face(client,n.x,n.z,-.1);await open(client,expected);
    }
    const production=require('./production-ui-helpers.cjs')({until,resume,station});
    const shot=(client,name)=>client.page.screenshot({path:path.join(out,name+'.png')});
    const contentActions=client=>client.page.locator('#modal-content [data-action]').evaluateAll(buttons=>[...new Set(buttons.map(button=>button.dataset.action))].sort());
    async function checkStaff(returning){
      // Teach the chef a recipe through a normal oven bake, then fund a pantry.
      await production.bake(returning,'berry_cookie',1);await until(()=>returning.snapshot.me.discovered.berry_cookie,'learn berry recipe');
      await management(returning);
      const hireCoins=returning.snapshot.me.coins;
      await returning.page.getByRole('button',{name:/^Hire chef/}).click();await until(()=>returning.snapshot.me.bakeryStaff.chef,'hire chef');
      await returning.page.getByRole('button',{name:/^Hire cashier/}).click();await until(()=>returning.snapshot.me.bakeryStaff.cashier,'hire cashier');await returning.page.getByRole('button',{name:/^Hire server/}).click();await until(()=>returning.snapshot.me.bakeryStaff.server,'hire server');assert.equal(returning.snapshot.me.coins,hireCoins-750);await shot(returning,'hired-team');
      assert.ok(returning.snapshot.nodes.some(n=>n.kind==='bakery_chef'));assert.ok(returning.snapshot.nodes.some(n=>n.kind==='bakery_cashier'));
      await station(returning,'oven',/YOUR CHEF'S RECIPE/);
      const berryChoice=returning.page.locator('.bakery-chef-choice').filter({hasText:'Berry burst'});await berryChoice.getByRole('button',{name:'Choose recipe',exact:true}).click();await until(()=>returning.snapshot.me.bakeryChefRecipe==='berry_cookie','assign chef recipe');
      await station(returning,'fridge',/KITCHEN FRIDGE/);
      const store=async item=>{const row=returning.page.locator('.shop-row').filter({has:returning.page.getByRole('heading',{name:item,exact:true})});await row.getByRole('button',{name:'Store 5',exact:true}).click();};
      await store('Cookie dough');const berriesBefore=returning.snapshot.me.inventory.berry;await store('Berry');await until(()=>returning.snapshot.me.inventory.berry===berriesBefore-5,'fund the pantry');
      const personal=structuredClone(returning.snapshot.me.inventory),automationCoins=returning.snapshot.me.coins;
      await shot(returning,'chef-pantry');await resume(returning);await route(returning,[[58,68.5],[58,73.5]]);await face(returning,63,75,-.03);
      await until(()=>returning.snapshot.me.bakeryPantry.berry<5,'chef consumes pantry ingredients',20000);
      assert.equal(returning.snapshot.me.inventory.berry,personal.berry);assert.equal(returning.snapshot.me.inventory.dough,personal.dough);await shot(returning,'chef-and-cashier-at-work');
      await until(()=>returning.snapshot.me.coins>automationCoins,'cashier automatically sells displayed goods',45000);
      assert.ok(returning.snapshot.me.bakeryBusiness.customers.some(c=>c.phase==='leaving'&&c.amount>0));await shot(returning,'automated-sale');await travel(returning,58,77);await face(returning,63,75,.02);await shot(returning,'cashier-at-the-display');await route(returning,[[58,73.5],[58,68.5]]);await face(returning,53,66.5,.02);await shot(returning,'chef-at-the-oven');
      console.log('PASS: hired visible chef/cashier, assigned known recipe, pantry-only production and automatic NPC sales');
    }
    const owner=await login(ownerEmail);assert.equal(owner.snapshot.me.id,ownerID);
    if(reopenOnly||staffOnly||buildingOnly||productionOnly||queuesOnly){
      const saved=structuredClone(owner.snapshot.me);assert.equal(saved.bakery,true);assert.equal(saved.bakeryPlot,'');
      await route(owner,[[-35,-35],[-35,-9],[8,-9],[8,84],[58,84],[58,78],[58,74],[58,72]]);await face(owner,58,68);await open(owner,/Claim this plot for free/);await shot(owner,'returning-free-claim');
      await owner.page.getByRole('button',{name:/^Claim plot/}).click();await until(()=>owner.snapshot.me.bakeryPlot==='bakery_plot_1','free claim');
      assert.equal(owner.snapshot.me.coins,saved.coins);assert.equal(owner.snapshot.me.bakeryLevel,saved.bakeryLevel);assert.deepEqual(owner.snapshot.me.equipment,saved.equipment);assert.deepEqual(owner.snapshot.me.paint,saved.paint);assert.equal(owner.snapshot.nodes.find(n=>n.id==='bakery_plot_1').owner,ownerID);await shot(owner,'saved-bakery-restored');
      if(queuesOnly){await require('./bakery-queue-checks.cjs')({visualOnly:queueVisual,owner,until,resume,face,travel,route,open,shot,station,plotOf,actions,out});assert.deepEqual(errors,[]);console.log('Screenshots: '+out);return;}
      if(productionOnly){await require('./production-browser-checks.cjs')({owner,login,ownerEmail,until,resume,face,travel,route,open,shot,management,station,plotOf,actions,out});assert.deepEqual(errors,[]);console.log('Screenshots: '+out);return;}
      if(buildingOnly){await require('./bakery-building-checks.cjs')({owner,login,ownerEmail,otherEmail,until,resume,face,travel,route,open,shot,management,station,plotOf,actions,out,bake:production.bake});assert.deepEqual(errors,[]);console.log('Screenshots: '+out);return;}
      await production.bake(owner,'sugar',1);await shot(owner,'reopened-oven');if(staffOnly)await checkStaff(owner);
      assert.deepEqual(errors,[]);console.log('PASS: saved bakery free claim preserves coins/upgrades/equipment/paint and reopened oven bakes through real UI');console.log('Screenshots: '+out);return;
    }
    await route(owner,[[-35,-35],[-35,-9],[8,-9],[8,84],[24,84],[24,78],[24,74],[24,72]]);
    await face(owner,24,68);
    if(handoverOnly){await open(owner,/Claim this plot for free/);await owner.page.getByRole('button',{name:/^Claim plot/}).click();await until(()=>owner.snapshot.me.bakeryPlot==='bakery_plot_0','opening saved bakery fixture');assert.equal(owner.snapshot.me.coins,703);}
    else{
    await open(owner,/Buy your first bakery/);await shot(owner,'claim-first-bakery');
    await owner.page.getByRole('button',{name:/^Buy bakery/}).click();await until(()=>owner.snapshot.me.bakeryPlot==='bakery_plot_0','first bakery purchase');assert.equal(owner.snapshot.me.coins,800);
    await until(async()=>await owner.page.getByRole('button',{name:/^Upgrade/}).isVisible(),'management refreshed');
    await owner.page.getByRole('button',{name:/^Upgrade/}).click();await until(()=>owner.snapshot.me.bakeryLevel===1,'upgrade saved');assert.equal(owner.snapshot.me.coins,750);
    const walls=owner.page.locator('.shop-row').filter({has:owner.page.getByRole('heading',{name:'Walls',exact:true})});await walls.getByRole('button',{name:'Teal',exact:true}).click();await until(()=>owner.snapshot.me.paint.walls==='teal','paint saved');assert.ok((await contentActions(owner)).includes('bakery_hire'));await shot(owner,'manage-bakery');
    await owner.page.locator('#bakery-name').fill('Moonflower Bakery');
    await owner.page.locator('#bakery-name-form').getByRole('button',{name:'Save name',exact:true}).click();
    await until(()=>owner.snapshot.me.bakeryName==='Moonflower Bakery','bakery name saved');
    await until(()=>owner.snapshot.nodes.find(node=>node.id==='bakery_plot_0')?.label==='Moonflower Bakery','bakery sign reflects saved name');
    await shot(owner,'renamed-bakery');
    console.log('PASS: first bakery purchase, upgrade, paint, and rename via UI');
    await station(owner,'oven',/Hold plain dough/);
    assert.ok(!(await contentActions(owner)).includes('craft'));assert.equal(await owner.page.getByRole('button',{name:'Build campfire',exact:true}).count(),0);assert.ok((await contentActions(owner)).includes('production_bake'));
    await production.bake(owner,'sugar',1);await shot(owner,'oven-recipes-only');
    await station(owner,'mixer',/MAKE PLAIN DOUGH/);assert.ok((await contentActions(owner)).includes('production_mix'));assert.ok(!(await contentActions(owner)).includes('bakery_fridge_deposit'));
    const beforeDough=owner.snapshot.me.inventory.dough;await production.click(owner,'production_mix',{item:'dough',amount:1});await until(()=>owner.snapshot.me.production?.bakery_mixer?.output?.dough===40,'timed mixer dough: '+owner.lastError,48000);await production.collect(owner,'mixer','dough');assert.equal(owner.snapshot.me.inventory.dough,beforeDough+40);await shot(owner,'mixer-dough-only');
    await station(owner,'display',/Put your baked goods on the display/);
    const beforeCoins=owner.snapshot.me.coins,beforeStock=owner.snapshot.me.inventory.sugar;
    await owner.page.locator('[data-action="bakery_stock"]').first().click();await until(()=>owner.snapshot.me.bakeryDisplay.sugar===5,'stock the display');
    assert.equal(owner.snapshot.me.coins,beforeCoins,'stocking must not instantly pay');assert.equal(owner.snapshot.me.inventory.sugar,beforeStock-5);await shot(owner,'stocked-display');
    await until(()=>owner.snapshot.me.bakeryBusiness?.customers?.some(c=>c.phase==='waiting'),'NPC reaches the display',45000);
    const order=owner.snapshot.me.bakeryBusiness.customers.find(c=>c.phase==='waiting');assert.equal(order.amount,5);
    assert.ok(owner.snapshot.nodes.some(n=>n.kind==='bakery_customer'&&n.phase==='waiting'));
    await owner.page.locator('[data-action="bakery_serve"]').first().click();await until(()=>owner.snapshot.me.coins===beforeCoins+16,'manual customer payment: '+owner.lastError);assert.equal(owner.snapshot.me.bakeryDisplay.sugar||0,0);await shot(owner,'served-real-customer');
    console.log('PASS: separate stations, stocked display without instant payout, visible NPC customer and manual order sale');
    }
    const saved=structuredClone(owner.snapshot.me);await resume(owner);await route(owner,[[24,72],[24,74]]);
    await owner.page.setViewportSize({width:480,height:320});
    const neighbor=await login(otherEmail);await route(neighbor,[[-35,-35],[-35,-9],[8,-9],[8,84],[24,84],[24,78],[24,74],[24,72]]);await open(neighbor,/belongs to Bakery Owner/);await shot(neighbor,'occupied-plot');
    const context=owner.context;await owner.page.close();await until(()=>neighbor.snapshot.nodes.find(n=>n.id==='bakery_plot_0')?.owner==='','plot released on socket disconnect');
    await until(async()=>await neighbor.page.getByRole('button',{name:/^Buy bakery/}).isVisible(),'released plot panel refreshed');await neighbor.page.getByRole('button',{name:/^Buy bakery/}).click();await until(()=>neighbor.snapshot.me.bakeryPlot==='bakery_plot_0','second buyer claim');assert.equal(neighbor.snapshot.me.coins,750);await shot(neighbor,'neighbor-claims-released-plot');
    console.log('PASS: immediate socket release and second-account purchase');
    const returning=await login(ownerEmail,context);assert.equal(returning.snapshot.me.bakery,true);assert.equal(returning.snapshot.me.bakeryPlot,'');
    await route(returning,[[24,78],[24,84],[58,84],[58,78],[58,74],[58,72]]);await face(returning,58,68);await open(returning,/Claim this plot for free/);await shot(returning,'returning-free-claim');
    await returning.page.getByRole('button',{name:/^Claim plot/}).click();await until(()=>returning.snapshot.me.bakeryPlot==='bakery_plot_1','free claim');
    assert.equal(returning.snapshot.me.coins,saved.coins);assert.equal(returning.snapshot.me.bakeryLevel,saved.bakeryLevel);assert.deepEqual(returning.snapshot.me.equipment,saved.equipment);assert.deepEqual(returning.snapshot.me.paint,saved.paint);assert.equal(returning.snapshot.me.bakeryName,saved.bakeryName);
    assert.equal(returning.snapshot.nodes.find(n=>n.id==='bakery_plot_0').owner,otherID);assert.equal(returning.snapshot.nodes.find(n=>n.id==='bakery_plot_1').owner,ownerID);await shot(returning,'saved-bakery-restored');
    await production.bake(returning,'sugar',1);

    if(!handoverOnly)await checkStaff(returning);

    assert.deepEqual(errors,[]);console.log(handoverOnly?'PASS: real movement, immediate socket release, second buyer, free saved-bakery claim preserving upgrades/equipment/paint, and reopened oven':'PASS: real movement, first claim, upgrade/paint, separate oven/mixer/display menus and actions, socket release, second buyer, free saved-bakery claim');console.log('Screenshots: '+out);
  }catch(error){if(failurePage&&!failurePage.isClosed())await failurePage.screenshot({path:path.join(out,'failure.png')}).catch(()=>{});if(failurePage&&!failurePage.isClosed())console.error('UI state',await failurePage.evaluate(()=>window.__bakeryDebug?.()).catch(()=>null));throw error;}
  finally{if(browser)await browser.close();server.kill();fs.writeFileSync(path.join(out,'server.log'),log);fs.writeFileSync(path.join(out,'actions.json'),JSON.stringify(actions,null,2));}
})().catch(error=>{console.error(error);process.exitCode=1;});
