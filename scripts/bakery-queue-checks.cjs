// Real UI purchases, held-ingredient queues, independent stations and direct service.
const assert=require('node:assert/strict');
module.exports=async function(deps){
  const {owner:client,until,resume,face,travel,route,open,shot,station,plotOf,actions}=deps;
  const {click,batch,hold}=require('./production-ui-helpers.cjs')(deps),plot=plotOf(client);
  if(deps.visualOnly){
    await station(client,'mixer',/NEXT BATCHES/);await client.page.locator('#modal-content').evaluate(el=>el.scrollTop=0);await shot(client,'visible-active-and-waiting-queue');
    await hold(client,'dough');await station(client,'cooling_table',/FINISHED BAKES/);await resume(client);await face(client,plot.x-1.8,plot.z-5.5,-.36);await shot(client,'cooling-table-visible-cookies');
    console.log('PASS: queue and cooling-table visual capture from an isolated saved fixture');return;
  }
  const furniture=()=>client.snapshot.me.bakeryFurniture||[],nodeFor=f=>client.snapshot.nodes.find(n=>n.furnitureId===f.id&&n.kind==='bakery_'+f.kind);
  const originals={oven:furniture().find(f=>f.kind==='oven').id,mixer:furniture().find(f=>f.kind==='mixer').id};
  async function mode(enabled){await resume(client);if(!!client.snapshot.me.bakeryBuildMode!==enabled)await client.page.keyboard.press('b');await until(()=>!!client.snapshot.me.bakeryBuildMode===enabled,'bakery build mode');}
  async function buy(kind,x,z){
    await resume(client);await client.page.keyboard.press('6');await client.page.locator('#modal').waitFor({state:'visible'});
    const before=furniture().length,coins=client.snapshot.me.coins,cost=client.snapshot.bakeryFurnitureCatalog.find(r=>r.id===kind).cost;
    await click(client,'bakery-place-choice',{item:kind});await resume(client);
    const wx=plot.x+x,wz=plot.z+z,distance=Math.hypot(wx-client.snapshot.me.x,wz-client.snapshot.me.z);await face(client,wx,wz,-Math.atan2(1.7,distance));
    await client.page.waitForFunction(({x,z})=>{const b=window.__bakeryDebug?.().building;return b&&b.valid&&b.x===x&&b.z===z;},{x,z});await client.page.mouse.down();await client.page.mouse.up();
    await until(()=>furniture().length===before+1,'place additional '+kind+': '+client.lastError);assert.equal(client.snapshot.me.coins,coins-cost);return furniture().find(f=>f.kind===kind&&f.id!==originals[kind]);
  }
  async function appliance(f){
    const node=nodeFor(f);assert.ok(node,'station node matches its furnishing');
    await route(client,[[plot.x,plot.z-3.5],[node.x,node.z+1.8]]);await face(client,node.x,node.z,-.1);await open(client,/NEXT BATCHES/);
    assert.ok((await client.page.locator('[data-production-queue]').getAttribute('data-production-queue'))===node.id,'opening exact appliance');return node;
  }
  await route(client,[[plot.x,plot.z-3.5],[plot.x-1.5,plot.z-3.5]]);await mode(true);
  const extraOven=await buy('oven',-7.5,-5.5);await travel(client,plot.x,plot.z-3.5);const extraMixer=await buy('mixer',2.5,-5.5);
  await client.page.keyboard.press('6');await client.page.locator('#modal').waitFor({state:'visible'});await shot(client,'multiple-appliance-catalog');await mode(false);
  assert.equal(furniture().filter(f=>f.kind==='oven').length,2);assert.equal(furniture().filter(f=>f.kind==='mixer').length,2);
  const mixer=await appliance(extraMixer),mixerKey=mixer.productionKey;assert.notEqual(mixerKey,'bakery_mixer');
  await click(client,'production_mix',{target:mixer.id,item:'dough',amount:1});await until(()=>client.snapshot.me.production[mixerKey]?.job?.item==='dough','extra mixer active');
  await hold(client,'dough');await batch(client,5);await click(client,'production_mix',{target:mixer.id,item:'berry_cookie',amount:5});await until(()=>client.snapshot.me.production[mixerKey]?.queue?.length===1,'first queued recipe');
  await batch(client,1);await click(client,'production_mix',{target:mixer.id,item:'berry_cookie',amount:1});await until(()=>client.snapshot.me.production[mixerKey]?.queue?.length===2,'second queued recipe');
  assert.deepEqual(client.snapshot.me.production[mixerKey].queue.map(j=>[j.item,j.amount]),[['prepared_berry_cookie',5],['prepared_berry_cookie',1]]);assert.equal(client.snapshot.me.inventory.dough,24);assert.equal(client.snapshot.me.inventory.berry,8);await client.page.locator('#modal-content').evaluate(el=>el.scrollTop=0);await shot(client,'mixer-fifo-queue');
  await station(client,'mixer',/MAKE PLAIN DOUGH/);await click(client,'production_mix',{item:'dough',amount:1});await until(()=>client.snapshot.me.production.bakery_mixer?.job,'original mixer also active');assert.ok(client.snapshot.me.production[mixerKey].job,'both mixers work concurrently');
  await station(client,'oven',/Hold plain dough/);await hold(client,'dough');await batch(client,1);await click(client,'production_bake',{item:'dough',amount:1});await until(()=>client.snapshot.me.production.bakery_oven?.job,'original oven active');
  await click(client,'production_bake',{item:'dough',amount:1});await until(()=>client.snapshot.me.production.bakery_oven?.queue?.length===1,'oven accepts waiting batch');await shot(client,'oven-fifo-queue');
  const oven=await appliance(extraOven),ovenKey=oven.productionKey;await batch(client,5);await click(client,'production_bake',{target:oven.id,item:'dough',amount:5});await until(()=>client.snapshot.me.production[ovenKey]?.job,'extra oven active');
  assert.equal(client.snapshot.me.production[ovenKey].job.amount,5);assert.equal(client.snapshot.me.production[ovenKey].job.destination,'bakery_cooling_table');await resume(client);await face(client,plot.x-4,plot.z-5.5,-.1);await shot(client,'multiple-appliances-working');
  await until(()=>client.snapshot.me.production.bakery_cooling_table?.output?.sugar===7,'independent ovens deliver all seven goods to shared table',40000);
  await until(()=>client.snapshot.me.production[mixerKey]?.output?.dough===40&&client.snapshot.me.production[mixerKey]?.output?.prepared_berry_cookie===6,'mixer FIFO finishes without overwriting output',50000);
  assert.equal(client.snapshot.me.production[mixerKey].queue.length,0);assert.ok(!client.snapshot.me.production[mixerKey].job);await until(()=>client.snapshot.me.production.bakery_mixer.output.dough===40,'later-started original mixer finishes independently',15000);
  await station(client,'cooling_table',/FINISHED BAKES/);await resume(client);await face(client,plot.x-1.8,plot.z-5.5,-.28);await shot(client,'queued-cookies-on-shared-cooling-table');await open(client,/FINISHED BAKES/);await click(client,'production_collect',{item:'sugar',amount:7});await until(()=>client.snapshot.me.inventory.sugar===17,'collect all real queued goods');
  await station(client,'display',/Put your baked goods on the display/);await click(client,'bakery_stock',{item:'sugar',amount:5});await until(()=>client.snapshot.me.bakeryDisplay.sugar===5,'stock display for direct service');
  await until(()=>client.snapshot.me.bakeryBusiness?.customers?.some(c=>c.phase==='waiting'),'customer waiting',45000);
  const order=client.snapshot.me.bakeryBusiness.customers.find(c=>c.phase==='waiting'),customer=client.snapshot.nodes.find(n=>n.id===order.id);
  await route(client,[[plot.x,plot.z+5.5],[customer.x-2,customer.z+.5]]);const distance=Math.hypot(customer.x-client.snapshot.me.x,customer.z-client.snapshot.me.z);await face(client,customer.x,customer.z,-Math.atan2(.55,distance));await shot(client,'customer-before-direct-click');
  const throws=actions.filter(a=>a.action==='throw').length,coins=client.snapshot.me.coins;await client.page.mouse.down();await client.page.mouse.up();await client.page.locator('#modal.customer-popup').waitFor({state:'visible'});
  assert.match(await client.page.locator('#modal-title').innerText(),/Serve customer/);assert.ok((await client.page.locator('.customer-popup .modal-shell').boundingBox()).width<=420);assert.equal(await client.page.locator('[data-action="bakery-serve-customer"]').count(),1);assert.equal(actions.filter(a=>a.action==='throw').length,throws);await shot(client,'direct-customer-serve-popup');
  await click(client,'bakery-serve-customer',{target:customer.id});await until(()=>client.snapshot.me.coins===coins+order.price,'direct customer sale');assert.equal(client.snapshot.me.bakeryDisplay.sugar||0,5-order.amount);assert.equal(actions.filter(a=>a.action==='throw').length,throws);await shot(client,'customer-served-directly');
  console.log('PASS: purchased extra appliances, exact independent station targets, FIFO queues and upfront ingredients, shared cooling output, and direct customer click/Serve without throwing');
};
