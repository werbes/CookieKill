// Invoked by bakery-browser-smoke.cjs --building-only against its isolated server.
const assert=require('node:assert/strict');
module.exports=async function({owner,login,ownerEmail,until,resume,face,travel,route,open,shot,management,station,plotOf,bake}){
  let client=owner;
  const furniture=()=>client.snapshot.me.bakeryFurniture||[];
  const find=kind=>furniture().find(f=>f.kind===kind);
  const local=(x,z)=>{const p=plotOf(client);return [p.x+x,p.z+z];};
  const move=(x,z)=>travel(client,...local(x,z));
  async function mode(enabled){await resume(client);if(!!client.snapshot.me.bakeryBuildMode!==enabled)await client.page.keyboard.press('b');await until(()=>!!client.snapshot.me.bakeryBuildMode===enabled,'bakery mode '+enabled);}
  async function select(object){await resume(client);const [x,z]=local(object.x,object.z),distance=Math.hypot(x-client.snapshot.me.x,z-client.snapshot.me.z);await face(client,x,z,-Math.atan2(.75,distance));await client.page.keyboard.press('e');await until(()=>client.snapshot.me.editingBakeryFurnitureId===object.id,'select '+object.kind+': '+client.lastError);}
  async function place(x,z){await resume(client);const [wx,wz]=local(x,z),distance=Math.hypot(wx-client.snapshot.me.x,wz-client.snapshot.me.z);assert.ok(distance>=2.5&&distance<=7.2,'placement within camera range');await face(client,wx,wz,-Math.atan2(1.7,distance));await client.page.waitForFunction(({x,z})=>{const b=window.__bakeryDebug?.().building;return b&&b.valid&&b.x===x&&b.z===z;},{x,z});await client.page.mouse.down();await client.page.mouse.up();}
  async function catalog(){await resume(client);await client.page.keyboard.press('6');await client.page.locator('#modal').waitFor({state:'visible'});await until(async()=>/BAKERY FURNISHINGS/.test(await client.page.locator('#modal-content').innerText()),'bakery catalog');}
  async function choose(name){await catalog();await client.page.getByRole('button',{name,exact:true}).click();await client.page.waitForFunction(()=>!!document.pointerLockElement);}

  const plot=plotOf(client),layout=client.snapshot.layout.plots.find(p=>p.id===plot.id);
  assert.equal(layout.width,20);assert.equal(layout.depth,16);
  assert.equal(furniture().filter(f=>f.kind==='table').length,2);assert.equal(furniture().filter(f=>f.kind==='chair').length,4);
  await resume(client);await move(0,6.5);await face(client,...local(-5,3),-.03);await shot(client,'larger-cafe-and-seating');
  await move(0,-3);await face(client,...local(-5,-5.5),-.06);await shot(client,'separate-working-kitchen');
  await move(0,3);await mode(true);await select(find('display'));
  await client.page.keyboard.press('r');await client.page.keyboard.press('r');await place(6,3);
  await until(()=>find('display').x===6&&find('display').z===3&&find('display').rotation===90,'rotate and move display: '+client.lastError);
  const publicDisplay=client.snapshot.nodes.find(n=>n.kind==='bakery_display'&&n.owner===client.snapshot.me.id);
  assert.equal(publicDisplay.x,plot.x+6);assert.equal(publicDisplay.yaw,Math.PI/2);
  await shot(client,'moved-working-display');

  await move(0,6);await move(3,6);
  const coins=client.snapshot.me.coins;
  await choose('Place Potted plant');await place(7,6);
  await until(()=>find('plant'),'purchase and place decor: '+client.lastError);assert.equal(client.snapshot.me.coins,coins-15);
  await select(find('plant'));await catalog();
  await client.page.locator('[data-action="bakery_paint"]').filter({hasText:'Teal'}).click();
  await until(()=>find('plant').color==='teal','paint furnishing');await shot(client,'bakery-furniture-catalog');
  await resume(client);await client.page.keyboard.press('f');
  await until(()=>!find('plant')&&client.snapshot.me.inventory['bakery_plant:teal']===1,'pack painted furnishing');
  await choose('Place packed furnishing');await place(7,6);
  await until(()=>find('plant')?.color==='teal'&&!client.snapshot.me.inventory['bakery_plant:teal'],'reuse packed furnishing with its color');assert.equal(client.snapshot.me.coins,coins-15);

  // Packing a working station really removes its interaction and retains goods.
  await move(0,6);await move(0,-3.5);const oven=find('oven'),stock=structuredClone(client.snapshot.me.bakeryDisplay),pantry=structuredClone(client.snapshot.me.bakeryPantry);
  await select(oven);await client.page.keyboard.press('f');await until(()=>!find('oven'),'pack oven');
  assert.ok(!client.snapshot.nodes.some(n=>n.id===plot.id+'_oven'&&n.available));assert.deepEqual(client.snapshot.me.bakeryDisplay,stock);assert.deepEqual(client.snapshot.me.bakeryPantry,pantry);
  await choose('Place packed furnishing');await place(oven.x,oven.z);await until(()=>!!find('oven'),'replace oven');
  await mode(false);
  const before=furniture().length;
  await client.page.evaluate(target=>window.__bakerySocket.send(JSON.stringify({type:'action',action:'bakery_destroy',target})),find('plant').id);
  await until(()=>/build mode/i.test(client.lastError||''),'server rejects editing outside build mode');assert.equal(furniture().length,before);
  const sugar=client.snapshot.me.inventory.sugar;await bake(client,'sugar',1);assert.ok(client.snapshot.me.inventory.sugar>sugar,'replaced oven remains usable');
  await station(client,'display',/Put your baked goods on the display/);
  const saleCoins=client.snapshot.me.coins;await client.page.locator('[data-action="bakery_stock"]').first().click();await until(()=>client.snapshot.me.bakeryDisplay.sugar===5,'stock relocated display');
  await until(()=>client.snapshot.me.bakeryBusiness?.customers?.some(c=>c.phase==='waiting'),'customer reaches relocated rotated display',60000);
  await client.page.locator('[data-action="bakery_serve"]').first().click();await until(()=>client.snapshot.me.coins>saleCoins,'sell from relocated display');await shot(client,'customer-uses-new-layout');

  const saved=structuredClone(furniture()),context=client.context;
  await resume(client);await route(client,[[plot.x,plot.z+5],[plot.x,plot.z+10]]);await client.page.close();
  client=await login(ownerEmail,context);await until(()=>client.snapshot.me.bakeryPlot==='','old plot released');
  await route(client,[[58,84],[24,84],[24,78],[24,73]]);await face(client,24,72);await open(client,/Claim this plot for free/);
  await client.page.getByRole('button',{name:/^Claim plot/}).click();await until(()=>client.snapshot.me.bakeryPlot==='bakery_plot_0','reopen decorated bakery on another plot');
  assert.deepEqual(furniture(),saved,'local furniture positions and colors persist on a different plot');
  assert.equal(client.snapshot.nodes.find(n=>n.id==='bakery_plot_0_display').x,30);
  await move(0,6);await move(3,6);await mode(true);await select(find('plant'));await client.page.keyboard.press('t');await until(()=>!find('plant'),'destroy selected furniture');await mode(false);
  await move(0,6);await move(0,10);await face(client,24,72,.13);await shot(client,'larger-bakery-exterior');
  console.log('PASS: two bakery rooms, B/E/R/place editing, buying decor, painting, packing/reuse, working relocated stations, customer routing, mode enforcement, saved layout across plots, and destroy control');
};
