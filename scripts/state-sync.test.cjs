const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {test} = require('node:test');

(async () => {
  // Load the same browser module without requiring a package-wide Node ESM mode.
  const source = fs.readFileSync(path.join(__dirname, '../web/state-sync.js'), 'utf8');
  const {createSnapshotDecoder} = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  const snapshot = (extra = {}) => ({
    type:'snapshot', seq:1, time:1, serverTime:1000,
    me:{id:'self', health:100, deaths:0, x:5, home:{x:1}, inventory:{wood:3, sugar:2}, buffs:{speed:4}, hotbar:[{item:'sugar', count:2}]},
    players:[{id:'friend', x:3, health:100, avatar:{skin:1, shirt:'hoodie'}}],
    nodes:[{id:'berry', available:true, x:4}], animals:[{id:'hare', health:10}],
    projectiles:[{id:'cookie', x:1}], homes:[{owner:'friend', safe:true, x:2}],
    recipes:[{id:'sugar', dough:2}], events:[{id:'event1', text:'Hello'}], layout:{version:2},
    ...extra
  });
  const delta = (extra = {}) => ({type:'delta', seq:2, base:1, time:2, serverTime:2000, ...extra});

  test('day cycle and John Pork replace through deltas and dawn returns him to bed', () => {
    const decoder=createSnapshotDecoder(),day={phase:'day',dayNumber:0,elapsed:719,remaining:1,dayDuration:720,nightDuration:360};
    decoder.apply(snapshot({dayCycle:day,johnPork:null}));
    const john={id:'john_pork',x:12,y:0,z:0,target:'self',island:'',inCave:false,eating:false};
    const night=decoder.apply(delta({dayCycle:{...day,phase:'night',elapsed:0,remaining:360},johnPork:john}));
    assert.equal(night.dayCycle.phase,'night');assert.equal(night.johnPork.target,'self');
    const moving=decoder.apply({type:'delta',seq:3,base:2,johnPork:{...john,x:.5,eating:true}});assert.equal(moving.johnPork.eating,true);assert.equal(moving.dayCycle.phase,'night');assert.equal(night.johnPork.x,12);
    const sleeper={id:'john_pork',x:220.2,y:1,z:-238.8,yaw:Math.PI,target:'',island:'',inCave:false,eating:false,sleeping:true};
    const dawn=decoder.apply({type:'delta',seq:4,base:3,dayCycle:{...day,dayNumber:1,elapsed:0,remaining:720},johnPork:sleeper});assert.deepEqual(dawn.johnPork,sleeper);assert.equal(dawn.dayCycle.phase,'day');
    const away=decoder.apply({type:'delta',seq:5,base:4,johnPork:null});assert.equal(away.johnPork,null,'world transition hides the sleeper');
  });

  test('club catalog stays available during spins and personal results replace atomically', () => {
    const decoder=createSnapshotDecoder(),catalog=[{id:'chocolate_shake',name:'Chocolate shake',price:7}];
    decoder.apply(snapshot({clubDrinks:catalog}));
    const result={id:5,stake:25,sector:6,multiplier:10,payout:250};
    const next=decoder.apply(delta({me:{clubResult:result}}));
    assert.deepEqual(next.clubDrinks,catalog);assert.deepEqual(next.me.clubResult,result);
    assert.throws(()=>decoder.apply({type:'delta',seq:3,base:2,clubDrinks:{}}),/array/);
    const cleared=decoder.apply({type:'delta',seq:3,base:2,clubDrinks:[],me:{clubResult:null}});
    assert.deepEqual(cleared.clubDrinks,[]);assert.equal(cleared.me.clubResult,null);
  });

  test('unchanged data survives deltas and the previous player remains available for damage/death comparisons', () => {
    const decoder = createSnapshotDecoder(), before = decoder.apply(snapshot());
    const after = decoder.apply(delta({me:{health:70, deaths:1}}));
    assert.equal(before.me.health, 100);
    assert.equal(before.me.deaths, 0);
    assert.equal(after.me.health, 70);
    assert.equal(after.me.deaths, 1);
    assert.equal(after.me.x, 5);
    for (const name of ['players','nodes','animals','projectiles','homes','recipes','events','layout']) assert.strictEqual(after[name], before[name]);
    assert.equal(after.type, 'snapshot');
    assert.equal(after.time, 2);
    assert.equal(after.serverTime, 2000);
    assert.equal(after.seq, 2);
    assert.equal('base' in after, false);
  });

  test('zero, false and null overwrite fields and nested maps/arrays replace rather than merge', () => {
    const decoder = createSnapshotDecoder(), before = decoder.apply(snapshot());
    const after = decoder.apply(delta({
      me:{x:0, home:null, inventory:{sugar:0}, buffs:{}, hotbar:[]},
      nodes:{upsert:[{id:'berry', available:false, x:0}]},
      players:{upsert:[{id:'friend', avatar:{skin:0}}]},
      homes:{upsert:[{owner:'friend', safe:false, x:0}]}
    }));
    assert.equal(after.me.x, 0);
    assert.equal(after.me.home, null);
    assert.deepEqual(after.me.inventory, {sugar:0});
    assert.deepEqual(after.me.buffs, {});
    assert.deepEqual(after.me.hotbar, []);
    assert.deepEqual(after.nodes, [{id:'berry', available:false, x:0}]);
    assert.deepEqual(after.players[0].avatar, {skin:0});
    assert.equal(after.players[0].health, 100);
    assert.deepEqual(after.homes, [{owner:'friend', safe:false, x:0}]);
    assert.deepEqual(before.me.inventory, {wood:3, sugar:2});
    assert.equal(before.nodes[0].available, true);
    assert.equal(before.players[0].avatar.shirt, 'hoodie');
    assert.equal(before.homes[0].safe, true);
  });

  test('entities join, update and leave using id or home owner', () => {
    const decoder = createSnapshotDecoder(), before = decoder.apply(snapshot());
    const after = decoder.apply(delta({
      players:{remove:['friend'], upsert:[{id:'new', x:8, health:100}]},
      nodes:{remove:['berry']}, animals:{remove:['hare']},
      projectiles:{remove:['cookie'], upsert:[{id:'throw', x:3}]},
      homes:{remove:['friend'], upsert:[{owner:'new', x:9}]}
    }));
    assert.deepEqual(after.players, [{id:'new', x:8, health:100}]);
    assert.deepEqual(after.nodes, []);
    assert.deepEqual(after.animals, []);
    assert.deepEqual(after.projectiles, [{id:'throw', x:3}]);
    assert.deepEqual(after.homes, [{owner:'new', x:9}]);
    assert.equal(before.players[0].id, 'friend');
    assert.equal(before.projectiles[0].id, 'cookie');
  });

  test('recipes and events replace old arrays and explicit null clears them', () => {
    const decoder = createSnapshotDecoder();
    decoder.apply(snapshot());
    const after = decoder.apply(delta({recipes:[{id:'new'}], events:[]}));
    assert.deepEqual(after.recipes, [{id:'new'}]);
    assert.deepEqual(after.events, []);
    const cleared = decoder.apply(delta({seq:3, base:2, recipes:null, events:null}));
    assert.deepEqual(cleared.recipes, []);
    assert.deepEqual(cleared.events, []);
  });

  test('null snapshot and change arrays are safe', () => {
    const decoder = createSnapshotDecoder();
    const before = decoder.apply(snapshot({players:null, nodes:null, animals:null, projectiles:null, homes:null, recipes:null, events:null}));
    for (const name of ['players','nodes','animals','projectiles','homes','recipes','events']) assert.deepEqual(before[name], []);
    assert.deepEqual(decoder.apply(delta({players:{upsert:null, remove:null}})).players, []);
  });

  test('home-island transitions replace permissions and objects and clear on returning to main', () => {
    const decoder = createSnapshotDecoder();
    const before=decoder.apply(snapshot({island:{owner:'self',builders:{friend:true},objects:[{id:'chair'}]},friends:[{id:'friend',visitors:true}],buildCatalog:[{id:'wall'}]}));
    const after=decoder.apply(delta({island:{owner:'self',builders:{},objects:[]},friends:[{id:'friend',visitors:false}]}));
    assert.deepEqual(after.island.builders, {});
    assert.deepEqual(after.island.objects, []);
    assert.equal(before.island.objects.length,1);
    assert.equal(after.friends[0].visitors,false);
    assert.strictEqual(after.buildCatalog,before.buildCatalog);
    const main=decoder.apply(delta({seq:3,base:2,island:null,me:{island:''},friends:[]}));
    assert.equal(main.island,null);
    assert.deepEqual(main.friends,[]);
  });

  test('bakery catalog survives deltas and furniture replacement clears moved or packed objects', () => {
    const decoder=createSnapshotDecoder();
    const first=decoder.apply(snapshot({bakeryFurnitureCatalog:[{id:'table',price:25}],nodes:[{id:'bakery_plot_0',bakeryFurniture:[{id:'table_1',kind:'table',x:4,z:3}]}]}));
    const moved=decoder.apply(delta({me:{bakeryBuildMode:true,editingBakeryFurnitureId:'table_1'},nodes:{upsert:[{id:'bakery_plot_0',bakeryFurniture:[{id:'table_1',kind:'table',x:-4,z:4,rotation:90}]}]}}));
    assert.strictEqual(moved.bakeryFurnitureCatalog,first.bakeryFurnitureCatalog);
    assert.equal(moved.nodes[0].bakeryFurniture[0].x,-4);
    assert.equal(first.nodes[0].bakeryFurniture[0].x,4);
    const packed=decoder.apply(delta({seq:3,base:2,me:{editingBakeryFurnitureId:''},nodes:{upsert:[{id:'bakery_plot_0',bakeryFurniture:[]}]},bakeryFurnitureCatalog:[{id:'table',price:30}]}));
    assert.deepEqual(packed.nodes[0].bakeryFurniture,[]);
    assert.equal(packed.bakeryFurnitureCatalog[0].price,30);
    const cleared=decoder.apply(delta({seq:4,base:3,bakeryFurnitureCatalog:null}));
    assert.deepEqual(cleared.bakeryFurnitureCatalog,[]);
  });

  test('ingredient catalog and timed station output survive and clear through deltas', () => {
    const decoder=createSnapshotDecoder(),catalog=[{id:'baking_sugar',name:'Sugar',price:4}];
    const first=decoder.apply(snapshot({bakingIngredients:catalog,me:{id:'self',production:{bakery_mixer:{job:{item:'dough',amount:40,remaining:40,total:40},output:{}}}}}));
    const ready=decoder.apply(delta({me:{production:{bakery_mixer:{output:{dough:40}}}}}));
    assert.strictEqual(ready.bakingIngredients,first.bakingIngredients);
    assert.equal(ready.me.production.bakery_mixer.job,undefined);
    assert.equal(ready.me.production.bakery_mixer.output.dough,40);
    assert.equal(first.me.production.bakery_mixer.job.remaining,40);
    const collected=decoder.apply(delta({seq:3,base:2,me:{production:{bakery_mixer:{output:{}}}},bakingIngredients:null}));
    assert.deepEqual(collected.me.production.bakery_mixer.output,{});
    assert.deepEqual(collected.bakingIngredients,[]);
  });

  test('out-of-order, skipped and stale updates reject without changing the baseline', () => {
    const decoder = createSnapshotDecoder();
    assert.throws(() => decoder.apply(delta()), /sequence/);
    decoder.apply(snapshot());
    for (const frame of [delta({base:0}), delta({seq:1}), delta({seq:3}), delta({seq:2.5}), snapshot()]) {
      assert.throws(() => decoder.apply(frame), /sequence/);
    }
    assert.equal(decoder.apply(delta({me:{health:80}})).me.health, 80);
    assert.throws(() => decoder.apply(delta()), /sequence/);
    assert.equal(decoder.apply(delta({seq:3, base:2})).me.health, 80);
  });

  test('malformed changes reject atomically', () => {
    const decoder = createSnapshotDecoder();
    decoder.apply(snapshot());
    assert.throws(() => decoder.apply(delta({me:{health:1}, nodes:{upsert:[{x:9}]}})), /identity/);
    const after = decoder.apply(delta());
    assert.equal(after.me.health, 100);
    assert.deepEqual(after.nodes, [{id:'berry', available:true, x:4}]);
  });

  test('a reconnect starts a fresh baseline and a later full snapshot replaces all state', () => {
    let decoder = createSnapshotDecoder();
    decoder.apply(snapshot());
    decoder.apply(delta({me:{health:5}}));
    decoder = createSnapshotDecoder();
    const fresh = decoder.apply(snapshot({me:{id:'self', health:100}, players:[], events:[]}));
    assert.equal(fresh.seq, 1);
    assert.equal(fresh.me.health, 100);
    assert.equal('inventory' in fresh.me, false);
    assert.deepEqual(fresh.players, []);
    assert.equal(decoder.apply(delta({me:{health:90}})).me.health, 90);
    const reset = decoder.apply(snapshot({seq:3, me:{id:'self', health:50}, nodes:[]}));
    assert.equal(reset.me.health, 50);
    assert.deepEqual(reset.nodes, []);
  });

  test('legacy full snapshots work and non-state messages do not change the sequence', () => {
    const decoder = createSnapshotDecoder(), first = snapshot();
    delete first.seq;
    assert.equal(decoder.apply(first).me.health, 100);
    assert.equal(decoder.apply({...first, me:{id:'self', health:90}}).me.health, 90);
    assert.throws(() => decoder.apply(delta()), /sequence/);
    decoder.apply(snapshot());
    assert.equal(decoder.apply({type:'error', message:'Cannot bake'}), null);
    assert.equal(decoder.apply(delta()).seq, 2);
  });
})().catch((error) => { console.error(error); process.exitCode = 1; });
