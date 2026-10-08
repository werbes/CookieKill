const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {test}=require('node:test');
const root=path.resolve(__dirname,'..');
const url=source=>'data:text/javascript;base64,'+Buffer.from(source).toString('base64');
const read=file=>fs.readFileSync(path.join(root,file),'utf8');
const settingsURL=url(read('web/club-settings.js')),oddsURL=url(read('web/club-odds.js'));
const clubSource=file=>read(file).replace('./club-settings.js',settingsURL).replace('./club-odds.js',oddsURL);
const modules=Promise.all([import(url(read('web/vendor/three.core.js'))),import(url(clubSource('web/club-scenery.js'))),import(url(clubSource('web/club-panels.js'))),import(settingsURL)]);

test('wheel slice sizes, pointers and displayed odds match the requested probabilities',async()=>{
  const {CLUB_MULTIPLIERS,CLUB_CHANCES,clubSectorCenter}=await import(oddsURL);
  assert.deepEqual(CLUB_MULTIPLIERS,[5,3,2,1,.5,.1,10]);
  assert.deepEqual(CLUB_CHANCES,[7,10,10,10,30,30,3]);
  assert.equal(CLUB_CHANCES.reduce((sum,n)=>sum+n,0),100);
  const [THREE,{buildForestClub}]=await modules,scene=buildForestClub({THREE,parent:new THREE.Group(),flatLabel:()=>new THREE.Group()});
  const wheel=scene.group.getObjectByName('club-fortune-wheel'),wedges=wheel.children.filter(o=>o.geometry?.type==='CircleGeometry'&&o.geometry.parameters.radius===1.35);
  assert.equal(wedges.length,7);
  for(let sector=0;sector<7;sector++){
    assert.ok(Math.abs(wedges[sector].geometry.parameters.thetaLength-Math.PI*2*CLUB_CHANCES[sector]/100)<1e-10);
    scene.update(sector*5,true,{id:sector+1,sector});scene.update(sector*5+3,true,{id:sector+1,sector});
    const center=wheel.rotation.z+clubSectorCenter(sector)*Math.PI*2;
    assert.ok(Math.abs(Math.sin(center)-1)<1e-10,'winning sector should point upwards');
  }
});

test('club entrance clips exactly its aperture after the desert cut, preserving terrain colors',async()=>{
  const [THREE,{cutClubOpening,CLUB_OPENING}]=await modules;
  const {cutCaveOpening}=await import(url(read('web/cave-scenery.js')));
  for(const indexed of [true,false]){
    let geometry=new THREE.PlaneGeometry(480,460,37,31);geometry.rotateX(-Math.PI/2);geometry.translate(0,0,-30);
    const position=geometry.attributes.position;geometry.setAttribute('color',new THREE.Float32BufferAttribute(Array.from({length:position.count},(_,i)=>[(position.getX(i)+240)/480,(position.getZ(i)+260)/460,.5]).flat(),3));
    if(!indexed)cutCaveOpening(geometry,THREE);
    cutClubOpening(geometry,THREE);
    const p=geometry.attributes.position,c=geometry.attributes.color;
    assert.equal(p.count,c.count);assert.equal(p.count,geometry.attributes.normal.count);
    let area=0;for(let i=0;i<p.count;i+=3){const ax=p.getX(i),az=p.getZ(i),bx=p.getX(i+1),bz=p.getZ(i+1),cx=p.getX(i+2),cz=p.getZ(i+2);area+=Math.abs((bx-ax)*(cz-az)-(bz-az)*(cx-ax))/2;for(let j=0;j<3;j++){assert.ok(Math.abs(c.getX(i+j)-(p.getX(i+j)+240)/480)<2e-6);assert.ok(Math.abs(c.getY(i+j)-(p.getZ(i+j)+260)/460)<2e-6);}}
    assert.ok(Math.abs(480*460-area-(indexed?60:120))<.003,'only the two intended hidden holes may lose terrain');
    const terrain=new THREE.Mesh(geometry,new THREE.MeshBasicMaterial({side:THREE.DoubleSide})),ray=new THREE.Raycaster();terrain.updateMatrixWorld();
    const hit=(x,z)=>{ray.set(new THREE.Vector3(x,10,z),new THREE.Vector3(0,-1,0));return ray.intersectObject(terrain).length>0;};
    for(const [x,z] of [[-202,-212],[-204.99,-207.01],[-199.01,-216.99]])assert.equal(hit(x,z),false);
    for(const [x,z] of [[CLUB_OPENING.minX-.01,-212],[CLUB_OPENING.maxX+.01,-212],[-202,CLUB_OPENING.minZ-.01],[-202,CLUB_OPENING.maxZ+.01]])assert.equal(hit(x,z),true);
  }
});

test('club scene stairs follow all 48 authoritative treads and the entrance stays open',async()=>{
  const [THREE,{buildForestClub,forestClubGround,forestClubFootprint,CLUB_STEPS,CLUB_RUN}]=await modules;
  const parent=new THREE.Group(),scene=buildForestClub({THREE,parent,flatLabel:()=>new THREE.Group()});
  scene.group.updateMatrixWorld(true);const ray=new THREE.Raycaster();
  for(let i=0;i<CLUB_STEPS;i++){
    const z=-207-(i+.5)*CLUB_RUN,y=-(i+1)*.25;
    assert.equal(forestClubGround(-202,z),y);assert.equal(forestClubFootprint(-202,z),true);
    ray.set(new THREE.Vector3(-202,y+.1,z),new THREE.Vector3(0,-1,0));const first=ray.intersectObject(scene.group,true)[0];assert.ok(first&&Math.abs(first.point.y-y)<.025,`rendered tread ${i} disagrees with ground ${y}`);
  }
  assert.equal(forestClubGround(-213,-246),-12);assert.equal(forestClubFootprint(-219,-246),false);
  ray.set(new THREE.Vector3(-202,1.7,-201.5),new THREE.Vector3(0,0,-1));ray.far=5.4;assert.equal(ray.intersectObject(scene.group,true).length,0,'rock mouth blocks the way onto the stairs');
  let staticBatches=0,tiles=0;scene.group.traverse(o=>{if(o.name.startsWith('club-static-'))staticBatches++;if(o.name==='club-disco-floor')tiles=o.count;});assert.ok(staticBatches<=6);assert.equal(tiles,80);
  scene.update(1,false);assert.ok(scene.lights.every(light=>light.intensity===0));scene.update(1,true);assert.ok(scene.lights.some(light=>light.intensity>0));
});

test('reduced flashing respects the device preference and freezes light animation',async()=>{
  const [THREE,{buildForestClub},,{clubReducedFlashing,setClubReducedFlashing}]=await modules;
  const preference={matches:true};globalThis.matchMedia=()=>preference;setClubReducedFlashing(false);assert.equal(clubReducedFlashing(),true);
  const scene=buildForestClub({THREE,parent:new THREE.Group(),flatLabel:()=>new THREE.Group()});scene.update(5,true);const floor=scene.group.getObjectByName('club-disco-floor'),colors=Array.from(floor.instanceColor.array);scene.update(17,true);assert.deepEqual(Array.from(floor.instanceColor.array),colors);assert.equal(scene.group.getObjectByName('club-mirror-ball').rotation.y,0);
  preference.matches=false;setClubReducedFlashing(true);assert.equal(clubReducedFlashing(),true);setClubReducedFlashing(false);assert.equal(clubReducedFlashing(),false);delete globalThis.matchMedia;
});

test('Disco Duke and his cake stand appear at night at the authoritative seller location',async()=>{
  const [THREE,{buildForestClub}]=await modules;
  const parent=new THREE.Group(),scene=buildForestClub({THREE,parent,flatLabel:()=>new THREE.Group(),makePerson:()=>new THREE.Group()});
  const seller=scene.group.getObjectByName('club-cake-seller');
  assert.ok(seller);assert.deepEqual(seller.position.toArray(),[-191,-12,-245]);assert.equal(seller.visible,false);
  scene.update(1,true,null,true);assert.equal(seller.visible,true);
  for(const part of ['coat','hat','feather','chain','cane','sign'])assert.ok(seller.getObjectByName('club-cake-seller-'+part),part+' should be visible at the stand');
  assert.ok(seller.getObjectByName('club-funny-cake-tray'));assert.equal(seller.getObjectsByProperty('name','club-funny-cake').length,5);
  scene.update(2,false,null,false);assert.equal(seller.visible,false,'dawn hides him even while the club is inactive');
  scene.update(3,false,null,true);assert.equal(seller.visible,true);
  assert.ok(seller.children.filter(o=>o.isPointLight).every(o=>o.visible===false),'a distant club does not keep the seller light active');
  parent.visible=false;assert.equal(seller.parent.parent,parent,'seller follows main-island scenery visibility');
});

test('club panels use authoritative drink prices and serialize spins through result/error/disconnect',async()=>{
  const [,,{createClubPanels,CLUB_MULTIPLIERS}]=await modules;
  const actions=[],state={connected:true,modal:'shop',context:{kind:'club_casino'},snapshot:{me:{coins:100,inventory:{chocolate_shake:2}},clubDrinks:[{id:'chocolate_shake',name:'Chocolate shake',price:7,heal:8,stamina:18}]}};
  let stake='10';const panel=createClubPanels({$:id=>id==='club-stake'?{value:stake}:null,state,action:(...args)=>actions.push(args),toast:()=>{},escapeHTML:value=>String(value).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),button:(label,kind,data,disabled)=>`<button data-action="${kind}"${disabled?' disabled':''}>${label}:${JSON.stringify(data)}</button>`,shopRow:(_i,name,desc,buttons)=>name+desc+buttons,renderModal:()=>{}});
  assert.deepEqual(CLUB_MULTIPLIERS,[5,3,2,1,.5,.1,10]);assert.match(panel.shop('club_bar'),/Chocolate shake/);assert.match(panel.shop('club_bar'),/7 coins/);assert.match(panel.shop('club_bar'),/2 in your inventory/);assert.equal(panel.drinkName('chocolate_shake'),'Chocolate shake');
  const event={target:{id:'club-spin-form'},preventDefault(){}};
  assert.match(panel.shop('club_casino'),/Jackpot chance: 3%/);assert.doesNotMatch(panel.shop('club_casino'),/2 in 7|equally likely/);stake='1.5';panel.submit(event);assert.equal(actions.length,0);
  stake='10';panel.submit(event);panel.submit(event);assert.equal(actions.length,1);assert.deepEqual(actions[0],['club_gamble',{amount:10,target:'club_casino'}]);assert.match(panel.shop('club_casino'),/Spinning/);
  state.snapshot.me.clubResult={id:1,stake:10,sector:5,multiplier:.1,payout:1};assert.match(panel.shop('club_casino'),/Net loss: 9 coins/);assert.doesNotMatch(panel.shop('club_casino'),/Waiting for your result/);
  await new Promise(resolve=>setTimeout(resolve,1110));panel.submit(event);assert.equal(actions.length,2);panel.actionFailed('Move closer to the wheel.');assert.match(panel.shop('club_casino'),/Move closer to the wheel/);assert.doesNotMatch(panel.shop('club_casino'),/Waiting for your result/);
  await new Promise(resolve=>setTimeout(resolve,1110));panel.submit(event);panel.connectionLost();assert.match(panel.shop('club_casino'),/Connection interrupted/);assert.doesNotMatch(panel.shop('club_casino'),/Waiting for your result/);
});
