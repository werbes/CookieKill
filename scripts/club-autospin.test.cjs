const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {test}=require('node:test');

const root=path.resolve(__dirname,'..');
const read=file=>fs.readFileSync(path.join(root,file),'utf8');
const url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const panels=import(url(read('web/club-panels.js')
  .replace('./club-settings.js',url(read('web/club-settings.js')))
  .replace('./club-odds.js',url(read('web/club-odds.js')))));
const escapeHTML=value=>String(value).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');

async function fixture(t){
  const {createClubPanels}=await panels;
  t.mock.timers.enable({apis:['Date','setTimeout'],now:10000});
  const actions=[];
  const state={
    connected:true,modal:'shop',context:{id:'club_casino',kind:'club_casino'},
    snapshot:{
      me:{coins:100,inCave:true,island:'',x:-191,z:-237,y:-12},
      nodes:[{id:'club_casino',kind:'club_casino',available:true,x:-191,z:-237,y:-12}],
    },
  };
  const stakeInput={id:'club-stake',value:'10'},submitButton={disabled:false};
  const panel=createClubPanels({
    $:id=>id==='club-stake'?stakeInput:id==='club-spin-form'?{querySelector:()=>submitButton}:null,
    state,action:(...args)=>actions.push(args),toast:()=>{},escapeHTML,
    button:(label,kind,_data,disabled)=>`<button data-action="${kind}"${disabled?' disabled':''}>${label}</button>`,
    shopRow:(_icon,name,description,buttons)=>name+description+buttons,
    renderModal:()=>{},
  });
  const view=()=>panel.shop('club_casino');
  const enabled=()=>{
    const input=view().match(/<input\b[^>]*\bid="club-auto-spin"[^>]*>/);
    assert.ok(input,'the fortune wheel should offer an auto-spin checkbox');
    return /\bchecked(?:\s|=|>)/.test(input[0]);
  };
  const toggle=checked=>panel.input({target:{id:'club-auto-spin',checked}});
  const result=id=>{
    state.snapshot.me.clubResult={id,stake:10,sector:3,multiplier:1,payout:10};
    panel.sync();
  };
  const manual=()=>panel.submit({target:{id:'club-spin-form'},preventDefault(){}});
  const stake=value=>{stakeInput.value=String(value);panel.input({target:stakeInput});};
  return {panel,state,actions,view,enabled,toggle,result,manual,stake,tick:ms=>t.mock.timers.tick(ms)};
}

test('auto-spin starts immediately, respects cooldown, and waits for each server result',async t=>{
  const f=await fixture(t);
  assert.match(f.view(),/Auto-spin/);
  assert.equal(f.enabled(),false);
  f.toggle(true);
  assert.equal(f.enabled(),true);
  assert.deepEqual(f.actions,[['club_gamble',{amount:10,target:'club_casino'}]]);
  f.panel.sync();f.panel.sync();
  assert.equal(f.actions.length,1,'repeated snapshots must not duplicate a pending spin');
  f.tick(550);f.result(1);f.tick(549);
  assert.equal(f.actions.length,1,'a result must not bypass the existing cooldown');
  f.tick(1);
  assert.equal(f.actions.length,2);
  f.tick(5500);f.panel.sync();f.tick(0);
  assert.equal(f.actions.length,2,'time alone must not repeat a spin awaiting its result');
  f.result(2);f.tick(0);
  assert.equal(f.actions.length,3,'a confirmed result permits the next eligible spin');
});

test('turning auto-spin off preserves an in-flight result and cancels future spins',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.toggle(false);
  assert.equal(f.enabled(),false);
  assert.match(f.view(),/Waiting for your result/);
  f.tick(2200);
  assert.equal(f.actions.length,1);
  f.result(1);f.tick(2200);
  assert.doesNotMatch(f.view(),/Waiting for your result/);
  assert.match(f.view(),/10 coins returned/);
  assert.equal(f.actions.length,1);
  f.toggle(true);
  assert.equal(f.actions.length,2,'the user can explicitly restart auto-spin');
});

test('turning auto-spin off after a result cancels the scheduled spin',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.result(1);f.tick(1099);f.toggle(false);f.tick(1);
  f.panel.sync();f.tick(2200);
  assert.equal(f.actions.length,1);
  assert.equal(f.enabled(),false);
});

test('repeated snapshots, renders, and a quick off/on toggle keep only one scheduled spin',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.result(1);
  for(let i=0;i<5;i++){f.panel.sync();f.view();}
  f.tick(500);f.toggle(false);f.toggle(true);
  for(let i=0;i<5;i++){f.panel.sync();f.view();}
  f.tick(600);
  assert.equal(f.actions.length,2);
  f.tick(2200);
  assert.equal(f.actions.length,2,'stale timers must not send while the new spin is pending');
  f.result(2);f.tick(0);
  assert.equal(f.actions.length,3);
});

test('enabling auto-spin during a manual spin does not send a second request',async t=>{
  const f=await fixture(t);
  f.manual();f.toggle(true);f.tick(2200);
  assert.equal(f.actions.length,1);
  f.result(1);f.tick(0);
  assert.equal(f.actions.length,2);
});

test('enabling auto-spin during the manual cooldown waits until the cooldown expires',async t=>{
  const f=await fixture(t);
  f.manual();f.result(1);f.toggle(true);f.tick(1099);
  assert.equal(f.actions.length,1);
  f.tick(1);
  assert.equal(f.actions.length,2);
});

test('leaving and reopening the wheel does not restart auto-spin',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.result(1);f.panel.stopAutoSpin();
  f.state.modal=null;f.state.context=null;f.panel.sync();f.tick(2200);
  f.state.modal='shop';f.state.context={id:'club_casino',kind:'club_casino'};
  f.panel.sync();f.tick(2200);
  assert.equal(f.actions.length,1);
  assert.equal(f.enabled(),false);
});

test('auto-spin permits the same horizontal and vertical range boundaries as the server',async t=>{
  const f=await fixture(t);
  f.state.snapshot.me.x=-196;f.state.snapshot.me.y=-10;
  f.toggle(true);
  assert.equal(f.actions.length,1);
  assert.equal(f.enabled(),true);
});

const ineligible=[
  ['moving beyond the wheel range',f=>{f.state.snapshot.me.x=-185.99;}],
  ['moving to another floor',f=>{f.state.snapshot.me.y=-9.99;}],
  ['leaving the underground',f=>{f.state.snapshot.me.inCave=false;}],
  ['travelling to an island',f=>{f.state.snapshot.me.island='home';}],
  ['a missing wheel',f=>{f.state.snapshot.nodes=[];}],
  ['an unavailable wheel',f=>{f.state.snapshot.nodes[0].available=false;}],
  ['falling over',f=>{f.state.snapshot.me.fallen=true;}],
  ['a disconnected session',f=>{f.state.connected=false;}],
  ['switching to another panel',f=>{f.state.modal='inventory';}],
  ['switching to another station',f=>{f.state.context={id:'club_bar',kind:'club_bar'};}],
  ['running out of stake money',f=>{f.state.snapshot.me.coins=9;}],
  ['reaching the exact-number wallet limit',f=>{f.state.snapshot.me.coins=Number.MAX_SAFE_INTEGER;}],
];

for(const [reason,invalidate] of ineligible){
  test(`auto-spin stops after ${reason} and stays off when eligibility returns`,async t=>{
    const f=await fixture(t);
    f.toggle(true);f.result(1);
    const eligible=structuredClone(f.state);
    invalidate(f);f.panel.sync();f.tick(2200);
    assert.equal(f.actions.length,1);
    assert.equal(f.enabled(),false);
    Object.assign(f.state,eligible);f.panel.sync();f.tick(2200);
    assert.equal(f.actions.length,1,'restoring eligibility must not silently resume auto-spin');
    assert.equal(f.enabled(),false);
  });
}

test('the scheduled spin checks range even before another explicit synchronization',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.result(1);
  f.state.snapshot.me.x=-180;
  f.tick(1100);
  assert.equal(f.actions.length,1);
  assert.equal(f.enabled(),false);
});

test('an invalid edited stake stops auto-spin until the user enables it again',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.result(1);f.stake('1.5');f.panel.sync();f.tick(2200);
  assert.equal(f.actions.length,1);
  assert.equal(f.enabled(),false);
  f.stake('5');f.panel.sync();f.tick(2200);
  assert.equal(f.actions.length,1);
  f.toggle(true);
  assert.deepEqual(f.actions[1],['club_gamble',{amount:5,target:'club_casino'}]);
});

test('a rejected auto-spin clears pending state and never retries automatically',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.panel.actionFailed('Move closer to the wheel.');
  assert.equal(f.enabled(),false);
  assert.match(f.view(),/Move closer to the wheel/);
  assert.doesNotMatch(f.view(),/Waiting for your result/);
  f.tick(2200);f.panel.sync();f.tick(2200);
  assert.equal(f.actions.length,1);
});

test('connection loss stops auto-spin before the connection flag changes and reconnect never resumes it',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.panel.connectionLost();
  assert.equal(f.enabled(),false);
  assert.match(f.view(),/Connection interrupted/);
  assert.doesNotMatch(f.view(),/Waiting for your result/);
  f.state.connected=false;f.tick(2200);
  f.state.connected=true;f.result(1);f.tick(2200);
  assert.equal(f.actions.length,1);
});

test('disconnecting between results cancels the waiting auto-spin timer',async t=>{
  const f=await fixture(t);
  f.toggle(true);f.result(1);f.panel.connectionLost();f.tick(2200);
  assert.equal(f.actions.length,1);
  assert.equal(f.enabled(),false);
});
