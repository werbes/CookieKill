const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {test}=require('node:test');
const root=path.resolve(__dirname,'..');
const read=file=>fs.readFileSync(path.join(root,file),'utf8');
const url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const panels=import(url(read('web/club-panels.js').replace('./club-settings.js',url(read('web/club-settings.js'))).replace('./club-odds.js',url(read('web/club-odds.js')))));
const escapeHTML=value=>String(value).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');

test('night seller uses server catalog and refuses unavailable, distant, unaffordable and disconnected purchases',async()=>{
  const {createClubPanels}=await panels,buttons=[];
  const state={connected:true,snapshot:{dayCycle:{phase:'night'},me:{x:-191,y:-12,z:-242.5,inCave:true,coins:30,inventory:{funny_cake:2}},nodes:[{id:'club_cake_seller',available:true,x:-191,y:-12,z:-245}],clubCakes:[{id:'funny_cake',name:'Funny cake',price:17,duration:49},{id:'cosmic_cake',name:'Cosmic cake',price:28,duration:81}]}};
  const panel=createClubPanels({$:()=>null,state,action:()=>{},toast:()=>{},escapeHTML,button:(label,action,payload,disabled)=>{buttons.push({label,action,payload,disabled:!!disabled});return label;},shopRow:(_icon,name,description,actions)=>name+' '+description+' '+actions,renderModal:()=>{}});
  const purchases=()=>{buttons.length=0;const text=panel.shop('club_cake_seller');return {text,buy:buttons.filter(b=>b.action==='buy_club_cake')};};
  let view=purchases();assert.match(view.text,/Disco Duke/);assert.match(view.text,/17 coins · 49 seconds/);assert.match(view.text,/2 in your inventory/);assert.match(view.text,/Eat one/);assert.equal(panel.cakeName('funny_cake'),'Funny cake');assert.equal(panel.drinkName('funny_cake'),undefined);
  assert.deepEqual(view.buy.map(b=>b.disabled),[false,false]);assert.deepEqual(view.buy[0].payload,{item:'funny_cake',amount:1,target:'club_cake_seller'});
  state.snapshot.me.coins=20;assert.deepEqual(purchases().buy.map(b=>b.disabled),[false,true]);
  state.snapshot.dayCycle.phase='day';view=purchases();assert.match(view.text,/Come back after sunset/);assert.ok(view.buy.every(b=>b.disabled));
  state.snapshot.dayCycle.phase='night';state.snapshot.nodes[0].available=false;assert.ok(purchases().buy.every(b=>b.disabled));
  state.snapshot.nodes[0].available=true;state.snapshot.me.x=-202;view=purchases();assert.match(view.text,/Come closer/);assert.ok(view.buy.every(b=>b.disabled));
  state.snapshot.me.x=-191;state.snapshot.me.y=0;assert.ok(purchases().buy.every(b=>b.disabled));
  state.snapshot.me.y=-12;state.snapshot.me.island='home';assert.ok(purchases().buy.every(b=>b.disabled));
  state.snapshot.me.island='';state.connected=false;assert.ok(purchases().buy.every(b=>b.disabled));
});

test('cake catalog survives deltas and an expired personal effect clears without affecting other players',async()=>{
  const {createSnapshotDecoder}=await import(url(read('web/state-sync.js'))),decoder=createSnapshotDecoder();
  const catalog=[{id:'funny_cake',name:'Funny cake',price:15,duration:45}],node={id:'club_cake_seller',available:true};
  const before=decoder.apply({type:'snapshot',seq:1,me:{id:'self',buffs:{funny_cake:45}},clubCakes:catalog,nodes:[node],players:[{id:'other',buffs:{funny_cake:30}}]});
  const after=decoder.apply({type:'delta',base:1,seq:2,me:{buffs:{}},nodes:{upsert:[{...node,available:false}]}});
  assert.deepEqual(after.clubCakes,catalog);assert.equal(after.me.buffs.funny_cake,undefined);assert.equal(after.players[0].buffs.funny_cake,30);assert.equal(before.me.buffs.funny_cake,45);assert.equal(after.nodes[0].available,false);
  assert.throws(()=>decoder.apply({type:'delta',base:2,seq:3,clubCakes:{}}),/array/);
  assert.deepEqual(decoder.apply({type:'delta',base:2,seq:3,clubCakes:[]}).clubCakes,[]);
});
