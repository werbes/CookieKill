const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),{test}=require('node:test');
const root=path.resolve(__dirname,'..'),load=file=>import('data:text/javascript;base64,'+Buffer.from(fs.readFileSync(path.join(root,file),'utf8')).toString('base64'));
const modules=Promise.all([load('web/vendor/three.core.js'),load('web/town-scenery.js'),load('web/town-panels.js')]);
const escapeHTML=value=>String(value).replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function fixture(createTownPanels){
  const flowerCatalog=[{id:'flower_daisy',name:'Daisy',sellPrice:3},{id:'flower_tulip',name:'Pink tulip',sellPrice:5},{id:'flower_lavender',name:'Lavender',sellPrice:4}];
  const state={connected:true,context:{id:'board_kills'},snapshot:{me:{id:'one',coins:100,x:168,y:0,z:155,inventory:{flower_daisy:2,flower_bouquet:1}},players:[],nodes:[{id:'flower_shop',kind:'flower_shop',x:168,z:153,available:true},{id:'betty',kind:'betty',x:171.8,z:154.5,available:true},{id:'pet_shop',kind:'pet_shop',x:198,z:153,available:true}],town:{flowerCatalog,bouquet:{id:'flower_bouquet',name:'A little sunshine',price:20},treatCatalog:[{id:'happy_apple',name:'Happy apple',price:10,stat:'happiness',gain:5,cap:100}],betty:{name:'Betty',greeting:'Hello, dear.',lines:[{id:'day',label:'How are you?',reply:'Lovely, dear. Glad you stopped by.'}]},leaderboards:[{id:'kills',name:'Most kills',unit:'kills',entries:[{rank:1,id:'two',name:'<Mia>',value:10}],self:{rank:12,id:'one',name:'Me',value:3},total:12}]}}};
  let renders=0;
  const button=(label,action,data={},disabled=false)=>`<button data-action="${action}" data-payload='${JSON.stringify(data)}'${disabled?' disabled':''}>${escapeHTML(label)}</button>`;
  const panel=createTownPanels({state,button,escapeHTML,shopRow:(_icon,title,description,buttons)=>`<article><h3>${escapeHTML(title)}</h3><p>${escapeHTML(description)}</p>${buttons}</article>`,renderModal:()=>{renders++;}});
  return {state,p:state.snapshot.me,panel,get renders(){return renders;}};
}
const buttons=(html,action)=>Array.from(html.matchAll(/<button data-action="([^"]+)" data-payload='([^']+)'([^>]*)>(.*?)<\/button>/g)).filter(match=>match[1]===action).map(match=>({data:JSON.parse(match[2]),disabled:match[3].includes('disabled'),label:match[4]}));

test('town renders five physical record boards, clear shop entrances, and flower nodes with authoritative availability',async()=>{
  const [THREE,{createTownScenery}]=await modules,parent=new THREE.Group();
  const scene=createTownScenery({THREE,parent,flatLabel:text=>{const g=new THREE.Group();g.userData.text=text;return g;}});
  for(const id of ['camel_coins','play_time','coins_earned','kills','cookie_ring_kills'])assert.ok(scene.group.getObjectByName('town-board-'+id));
  assert.ok(scene.group.getObjectByName('town-betty-label'));assert.equal(scene.group.children.filter(o=>o.isInstancedMesh).length,3,'town static geometry uses three batches');
  scene.group.updateMatrixWorld(true);const ray=new THREE.Raycaster();
  for(const [x,z,length] of [[130,163,15],[168,161,6.5],[198,161,6.5]]){
    ray.set(new THREE.Vector3(x,1.7,z),new THREE.Vector3(0,0,-1));ray.far=length;
    assert.equal(ray.intersectObject(scene.group,true).length,0,'shop doorway and main aisle are visually passable');
  }
  const snapshot={me:{},nodes:[{id:'stem',kind:'flower_tulip',x:149,y:0,z:145,available:true}],town:{leaderboards:[{id:'kills',unit:'kills',entries:[{rank:1,name:'Mia',value:7}]}]}};
  scene.update(snapshot);const flower=scene.group.getObjectByName('harvest-flower-stem');assert.deepEqual(flower.position.toArray(),[149,0,145]);assert.equal(flower.visible,true);
  assert.match(scene.group.getObjectByName('town-rank-kills-0').userData.text,/#1 Mia · 7/);
  snapshot.nodes[0].available=false;scene.update(snapshot);assert.equal(flower.visible,false,'picked stem vanishes');
  snapshot.nodes[0].available=true;scene.update(snapshot);assert.equal(flower.visible,true,'regrown stem returns');
  snapshot.me.inCave=true;scene.update(snapshot);assert.equal(scene.group.visible,false);snapshot.me={island:'one'};scene.update(snapshot);assert.equal(scene.group.visible,false);
  snapshot.me={};snapshot.nodes=[];scene.update(snapshot);assert.equal(flower.visible,false,'removed nodes do not leave phantom flowers');scene.dispose();assert.equal(parent.children.length,0);
});

test('flower shop uses live catalog prices, owned stems and nearby same-floor state',async()=>{
  const [,,{createTownPanels}]=await modules,{state,p,panel}=fixture(createTownPanels);
  const sale=buttons(panel.shop('flower_shop'),'sell_flower');assert.equal(sale.length,3);assert.deepEqual(sale[0].data,{target:'flower_shop',item:'flower_daisy',amount:1});assert.equal(sale[0].disabled,false);assert.equal(sale[1].disabled,true);
  assert.deepEqual(buttons(panel.shop('flower_shop'),'buy_bouquet')[0].data,{target:'flower_shop',item:'flower_bouquet',amount:1});
  p.coins=19;assert.equal(buttons(panel.shop('flower_shop'),'buy_bouquet')[0].disabled,true);
  p.coins=100;p.inCave=true;assert.equal(buttons(panel.shop('flower_shop'),'sell_flower')[0].disabled,true);
  p.inCave=false;p.x=180;assert.equal(buttons(panel.shop('flower_shop'),'buy_bouquet')[0].disabled,true);
  p.x=168;state.connected=false;assert.equal(buttons(panel.shop('flower_shop'),'buy_bouquet')[0].disabled,true);
  assert.equal(panel.itemName('flower_tulip'),'Pink tulip');assert.equal(panel.itemIcon('flower_bouquet'),'💐');assert.equal(panel.isTownItem('stone'),false);
});

test('gift picker only offers other players within four metres on the same floor',async()=>{
  const [,,{createTownPanels}]=await modules,{state,p,panel}=fixture(createTownPanels);
  state.snapshot.players=[{id:'one',name:'Self',x:168,z:155},{id:'near',name:'<Mia>',x:171.9,z:155},{id:'far',name:'Far',x:172.01,z:155},{id:'cave',name:'Cave',x:168,z:155,inCave:true},{id:'high',name:'High',x:168,z:155,y:3},{id:'other',name:'Island',x:168,z:155,island:'other'}];
  let gifts=buttons(panel.giftControls('flower_bouquet'),'give_bouquet');assert.equal(gifts.length,1);assert.equal(gifts[0].data.target,'near');assert.match(gifts[0].label,/&lt;Mia&gt;/);
  p.inventory.flower_bouquet=0;assert.equal(buttons(panel.giftControls('flower_bouquet'),'give_bouquet')[0].disabled,true);
  assert.equal(panel.giftControls('flower_daisy'),'');state.snapshot.players=[];assert.match(panel.giftControls('flower_bouquet'),/No one is close enough/);
});

test('Betty offers deterministic kind dialogue and board names are escaped',async()=>{
  const [,,{createTownPanels}]=await modules,fixtureValue=fixture(createTownPanels),{state,p,panel}=fixtureValue;
  assert.match(panel.shop('betty'),/Hello, dear/);assert.equal(panel.handle('town-betty-talk',{topic:'day'}),true);assert.equal(fixtureValue.renders,1);assert.match(panel.shop('betty'),/Glad you stopped by/);
  panel.handle('town-betty-talk',{topic:'<invalid>'});assert.equal(fixtureValue.renders,1);p.x=100;panel.handle('town-betty-talk',{topic:'day'});assert.equal(fixtureValue.renders,1);
  const boards=panel.shop('leaderboard_board');assert.match(boards,/&lt;Mia&gt;/);assert.doesNotMatch(boards,/<Mia>/);assert.match(boards,/Your rank: 12/);assert.match(boards,/gambling payouts/);
  assert.equal(panel.handle('unrelated',{}),false);assert.equal(panel.shop('unrelated'),null);
});

test('pet store serializes priced treats and disables purchases when out of reach',async()=>{
  const [,,{createTownPanels}]=await modules,{p,panel}=fixture(createTownPanels);p.x=198;
  const buy=buttons(panel.shop('pet_shop'),'buy_camel_treat')[0];assert.deepEqual(buy.data,{target:'pet_shop',item:'happy_apple',amount:1});assert.equal(buy.disabled,false);assert.match(panel.shop('pet_shop'),/2× happiness training progress/);assert.match(panel.shop('pet_shop'),/120 work per treat/);assert.doesNotMatch(panel.shop('pet_shop'),/\+5 happiness/);
  p.coins=9;assert.equal(buttons(panel.shop('pet_shop'),'buy_camel_treat')[0].disabled,true);p.coins=100;p.z=160;assert.equal(buttons(panel.shop('pet_shop'),'buy_camel_treat')[0].disabled,true);
});
