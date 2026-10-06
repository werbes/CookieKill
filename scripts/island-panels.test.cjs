const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');

(async () => {
  const source = fs.readFileSync(path.join(__dirname, '../web/island-panels.js'), 'utf8');
  const {createIslandPanels} = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  const actions = [], placements = [];
  const state = {snapshot:{me:{id:'owner',username:'baker',coins:200,friendCode:'FERN1234',inventory:{wood:3,sugar:8}},recipes:[{id:'sugar',name:'Sugar cookie'}],friends:[],buildCatalog:[{id:'wall',name:'Wall',category:'building',cost:{wood:4},width:4,depth:.4,height:3},{id:'fire',name:'Campfire',category:'appliances',cost:{wood:2},width:1.4,depth:1.4,height:.7}]}};
  const escapeHTML = value => String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const button = (label,kind,data={},disabled=false)=>`<button data-action="${kind}" data-payload="${escapeHTML(JSON.stringify(data))}"${disabled?' disabled':''}>${escapeHTML(label)}</button>`;
  const panel=createIslandPanels({
    $:()=>({value:'fern1234'}),state,action:(...args)=>actions.push(args),toast:()=>{},escapeHTML,pretty:value=>String(value).replaceAll('_',' '),button,
    shopRow:(_icon,label,description,buttons)=>`<article>${escapeHTML(label)}<p>${escapeHTML(description)}</p>${buttons}</article>`,
    openModal:(type,context)=>{state.modal=type;state.context=context;},closeModal:()=>{state.modal=null;},renderModal:()=>{},beginBuild:placement=>placements.push(placement)
  });
  const actionButton=(html,action)=>html.match(new RegExp(`<button data-action="${action}"[^>]*>`))?.[0];
  assert.match(panel.shop('pink_lollipop'),/Eat the pink lollipop/);
  assert.doesNotMatch(panel.shop('pink_lollipop'),/Buy your first land/);
  state.snapshot.me.homeOffer=true;
  assert.match(panel.shop('pink_lollipop'),/Buy your first land/);
  panel.handle('island-buy-home',{});
  assert.deepEqual(actions.at(-1),['buy_home']);
  assert.equal(actions.some(([action])=>action==='teleport_home'),false,'purchase should not teleport automatically');
  const home={owner:'owner',visitors:false,builders:{},chest:{wood:100},objects:[{id:'chest',kind:'chest',x:4,z:18},{id:'fire',kind:'fire',x:0,z:0,rotation:315,lit:true},{id:'pond',kind:'pond'}]};
  state.snapshot.me.homeIsland=home;
  assert.match(panel.shop('pink_lollipop'),/Would you like to teleport/);
  state.snapshot.island=home;Object.assign(state.snapshot.me,{island:'owner',x:0,z:20});
  assert.match(actionButton(panel.render('build'),'build-place'),/disabled/,'chest materials must be withdrawn before building');
  state.snapshot.me.inventory.wood=40;
  assert.doesNotMatch(actionButton(panel.render('build'),'build-place'),/disabled/);
  const chest=panel.render('chest');
  assert.match(chest,/Give 20/);assert.match(chest,/Take 20/);assert.doesNotMatch(chest,/sugar/);
  const amounts=[...chest.matchAll(/&quot;amount&quot;:(\d+)/g)].map(match=>Number(match[1]));
  assert.ok(amounts.length>0&&amounts.every(amount=>amount<=20));
  state.snapshot.me.x=25;
  assert.match(actionButton(panel.render('chest'),'chest_deposit'),/disabled/,'chest actions require proximity');
  state.context=home.objects[1];
  assert.match(panel.render('build_object'),/Put out campfire/);
  panel.handle('build-rotate',{id:'fire'});
  assert.equal(actions.at(-1)[1].rotation,0,'rotation wraps after eight 45 degree turns');
  panel.handle('build-move',{id:'fire'});
  assert.equal(placements.at(-1).object.id,'fire');
  state.snapshot.friends=[{id:'friend',username:'friend',callName:'<script>bad()</script>',request:true}];
  const friends=panel.render('friends');assert.match(friends,/&lt;script&gt;/);assert.doesNotMatch(friends,/<script>/);assert.match(friends,/friend_accept/);assert.match(friends,/friend_decline/);
  panel.submit({target:{id:'friend-request-form'},preventDefault(){}});
  assert.deepEqual(actions.at(-1),['friend_request',{target:'FERN1234'}]);
  state.snapshot.me.camelDiscount=true;
  assert.match(panel.shop('abu_fanous'),/Buy.*175 coins/);
  state.snapshot.island={...home,owner:'someone',builders:{},chest:{}};
  assert.match(actionButton(panel.render('build'),'build-place'),/disabled/,'a visitor needs explicit building permission');
  state.context={id:'fish',species:'fish',health:25,need:'plastic'};state.snapshot.animals=[state.context];
  assert.match(actionButton(panel.render('animal'),'adopt_animal'),/disabled/,'adoption is for wounds');
  state.context.need='wounded';assert.doesNotMatch(actionButton(panel.render('animal'),'adopt_animal'),/disabled/);
  console.log('PASS: island panels purchase, friends, permissions, chest, placement, camel, and adoption');
})().catch(error=>{console.error(error);process.exitCode=1;});
