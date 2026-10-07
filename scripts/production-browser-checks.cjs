const assert=require('node:assert/strict');
module.exports=async function(deps){
  const {until,resume,travel,route,face,open,shot,login,ownerEmail,station,plotOf}=deps;
  const {click,batch,hold,collect}=require('./production-ui-helpers.cjs')(deps);
  let client=deps.owner;
  assert.ok(client.snapshot.bakingIngredients.length===4);
  assert.ok(!client.snapshot.nodes.some(node=>node.kind==='dough'),'no diggable dough in the world');
  // Travel along city streets through the shop's open front entrance.
  await route(client,[[58,77],[58,84],[8,84],[8,34],[145,34],[145,30],[145,25],[145,22]]);await face(client,145,20);await open(client,/four basics for fresh dough/);
  const coins=client.snapshot.me.coins,baseCookies=client.snapshot.me.inventory.sugar;
  for(const item of ['baking_sugar','butter','flour','egg'])await click(client,'buy_ingredient',{item,amount:1});
  await until(()=>client.snapshot.me.inventory.egg===1,'shop purchases');assert.equal(client.snapshot.me.coins,coins-15);assert.equal(client.snapshot.me.inventory.sugar,baseCookies);await shot(client,'new-baking-ingredient-shop');
  await route(client,[[145,28],[145,34],[8,34],[8,84],[58,84],[58,76],[58,73]]);
  await station(client,'mixer',/MAKE PLAIN DOUGH/);const startingDough=client.snapshot.me.inventory.dough||0;
  const started=Date.now();await click(client,'production_mix',{item:'dough',amount:1});await until(()=>client.snapshot.me.production?.bakery_mixer?.job?.amount===40,'40 dough mixing starts');
  assert.equal(client.snapshot.me.production.bakery_mixer.job.total,40);assert.equal(client.snapshot.me.inventory.dough||0,startingDough);
  for(const item of ['baking_sugar','butter','flour','egg'])assert.equal(client.snapshot.me.inventory[item]||0,0);
  await shot(client,'forty-second-mixer');
  // Forged early collection cannot skip the timer or award anything.
  client.lastError='';await client.page.evaluate(target=>window.__bakerySocket.send(JSON.stringify({type:'action',action:'production_collect',target,item:'dough',amount:40})),plotOf(client).id+'_mixer');
  await until(()=>!!client.lastError,'premature collection rejected');assert.equal(client.snapshot.me.inventory.dough||0,startingDough);
  await until(()=>client.snapshot.me.production?.bakery_mixer?.output?.dough===40,'forty dough finish mixing',48000);
  assert.ok(Date.now()-started>=38500,'full batch was ready before40 seconds');assert.equal(client.snapshot.me.inventory.dough||0,startingDough,'finished mixer does not auto-award dough');
  await collect(client,'mixer','dough');assert.equal(client.snapshot.me.inventory.dough,startingDough+40);
  await hold(client,'dough');await shot(client,'holding-plain-dough');
  await batch(client,5);await click(client,'production_mix',{item:'berry_cookie',amount:5});await until(()=>client.snapshot.me.production?.bakery_mixer?.job?.item==='prepared_berry_cookie','held dough enters mixer');
  assert.equal(client.snapshot.me.production.bakery_mixer.job.total,5);assert.equal(client.snapshot.me.inventory.dough,startingDough+35);
  await until(()=>client.snapshot.me.production?.bakery_mixer?.output?.prepared_berry_cookie===5,'berry dough mixed',12000);await collect(client,'mixer','prepared_berry_cookie');
  await station(client,'oven',/Hold plain dough/);await hold(client,'prepared_berry_cookie');await batch(client,5);
  const beforeBaked=client.snapshot.me.inventory.berry_cookie||0;
  await click(client,'production_bake',{item:'prepared_berry_cookie',amount:5});await until(()=>client.snapshot.me.production?.bakery_oven?.job?.amount===5,'oven starts baking prepared dough');
  assert.equal(client.snapshot.me.production.bakery_oven.job.total,25);assert.equal(client.snapshot.me.inventory.berry_cookie||0,beforeBaked);await shot(client,'timed-oven-and-linked-table');
  await until(()=>client.snapshot.me.production?.bakery_cooling_table?.output?.berry_cookie===5,'finished cookies arrive at cooling table',33000);
  assert.equal(client.snapshot.me.inventory.berry_cookie||0,beforeBaked);assert.equal(client.snapshot.me.bakeryDisplay.berry_cookie||0,0);assert.equal(client.snapshot.me.discovered.berry_cookie,true);
  await station(client,'cooling_table',/FINISHED BAKES/);await shot(client,'finished-cookies-on-cooling-table');await collect(client,'cooling_table','berry_cookie');
  await station(client,'display',/Put your baked goods on the display/);await click(client,'bakery_stock',{item:'berry_cookie',amount:5});await until(()=>client.snapshot.me.bakeryDisplay.berry_cookie===5,'stock the actual finished cookies');
  const saleCoins=client.snapshot.me.coins;await until(()=>client.snapshot.me.bakeryBusiness?.customers?.some(c=>c.phase==='waiting'),'customer orders the baked batch',45000);await client.page.locator('[data-action="bakery_serve"]').first().click();await until(()=>client.snapshot.me.coins>saleCoins,'fresh cookie sale');
  // Start another oven, then reconnect; elapsed work/output must stay saved.
  await station(client,'oven',/Hold plain dough/);await hold(client,'dough');await batch(client,5);await click(client,'production_bake',{item:'dough',amount:5});await until(()=>!!client.snapshot.me.production?.bakery_oven?.job,'persistent oven job');
  const remaining=client.snapshot.me.production.bakery_oven.job.remaining,context=client.context;await client.page.close();await new Promise(resolve=>setTimeout(resolve,1000));client=await login(ownerEmail,context);
  assert.ok(client.snapshot.me.production.bakery_oven.job.remaining<=remaining);assert.ok(client.snapshot.me.production.bakery_oven.job.remaining>remaining-2,'offline oven timer advanced');
  const saved=client.snapshot.me.production.bakery_oven.job.remaining;await route(client,[[58,68.5],[58,73.2]]);await face(client,58,72);await open(client,/Claim this plot for free/);await client.page.getByRole('button',{name:/^Claim plot/}).click();await until(()=>client.snapshot.me.bakeryPlot==='bakery_plot_1','reclaim bakery resumes jobs');
  await until(()=>client.snapshot.me.production?.bakery_cooling_table?.output?.sugar===5,'resumed oven delivers saved batch',saved*1000+12000);await station(client,'cooling_table',/FINISHED BAKES/);await collect(client,'cooling_table','sugar');await shot(client,'saved-production-resumed');
  console.log('PASS: ingredient shop, exact40-dough recipe/40s timer, no early collection, held dough/add-ins, timed oven, linked-table output, displayed-cookie sale, and saved jobs across reconnect');
};
