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
