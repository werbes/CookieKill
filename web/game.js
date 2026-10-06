import * as THREE from './vendor/three.module.js';

const $ = (id) => document.getElementById(id);
const clamp = (n, a, b) => Math.min(b, Math.max(a, n));
const escapeHTML = (value) => String(value == null ? '' : value).replace(/[&<>"']/g, (c) => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const pretty = (key) => String(key).replaceAll('_', ' ').replace(/\b\w/g, (s) => s.toUpperCase());
const cookieOrder = ['sugar','berry_cookie','nut_cookie','cactus_cookie','sun_cookie','salt_cookie','protein_cookie','cake'];
const cookieNames = {sugar:'Sugar cookie',berry_cookie:'Berry cookie',nut_cookie:'Nut cookie',cactus_cookie:'Cactus cookie',sun_cookie:'Sun cookie',salt_cookie:'Sea salt cookie',protein_cookie:'Protein cookie',cake:'Cake'};
const cookieNotes = {sugar:'The classic',berry_cookie:'Slow your rivals',nut_cookie:'Weaken their throws',cactus_cookie:'A burst of speed',sun_cookie:'Go the distance',salt_cookie:'Made for the sea',protein_cookie:'10 seconds of power',cake:'A slice of chaos'};
const skinTones = ['#f7dcc5','#e7bd98','#c58d62','#936040','#583a2b'];
const garmentColors={teal:'#478b82',blue:'#607fa8',red:'#b75e50',purple:'#9177a7',sand:'#c9b88b',black:'#39403d',white:'#ece7d5'};
const avatarColor = (value,fallback) => garmentColors[value]||(/^#[0-9a-f]{6}$/i.test(value||'')?value:fallback);
const isKnown = (key) => !!state.snapshot?.me.discovered?.[key];
const cookieName = (key) => isKnown(key) ? cookieNames[key] : 'Unidentified cookie';
const cookieNote = (key) => isKnown(key) ? (cookieNotes[key] || '') : 'Bake this recipe to discover it';
const materialEmoji = {wood:'🪵',stick:'╱',stone:'◈',dough:'◕',berry:'●',nut:'◉',cactus:'♧',sea_salt:'✧',shell:'🐚',trash:'♻',pearl:'○',treasure:'♜',old_coin:'◎',protein_powder:'▥',protein_drink:'▤'};
const buffNames = {speed:'QUICK FEET',slow:'SLOWED',weaken:'WEAKER THROWS',weak:'WEAKER THROWS',range:'LONG THROW',short:'SHORT THROW',swim:'OCEAN LEGS',superstrength:'SUPERSTRENGTH',protein:'PROTEIN BOOST',training:'TRAINING BOOST'};
const state = {player:null,snapshot:null,ws:null,connected:false,playing:false,locked:false,modal:null,menuOpen:false,context:null,email:'',yaw:0,pitch:0,keys:new Set(),lastInput:0,selected:'sugar',selectedSlot:0,hotkeys:[],nodes:new Map(),animals:new Map(),players:new Map(),homes:new Map(),projectiles:new Map(),events:new Set(),nearest:null,lastHealth:100,localPosition:new THREE.Vector3(-35,1.7,-35),handSwing:0,lastThrow:0,lastSnapshot:0,hasPlayed:false,wasModalLocked:false,worldReady:false,serverRecipes:[],intentYaw:false};
let renderer, scene, camera, clock, worldGroup, hand, water, waterPositions, landingTime=0, lastUI=0, lastMinimap=0, lastFrame=0, lastAnimation=0, lastPanelRefresh=0, miniContext;
const dynamicGroup = new THREE.Group();
const materials = new Map();
const mat = (color, opts={}) => {
  const key = color + JSON.stringify(opts);
  if (!materials.has(key)) materials.set(key,new THREE.MeshStandardMaterial({color,roughness:.9,flatShading:true,...opts}));
  return materials.get(key);
};
const rng = (seed) => () => { seed = (seed * 1664525 + 1013904223) >>> 0; return seed / 4294967296; };
const random = rng(2361);
const mesh = (geometry,color,x=0,y=0,z=0,parent=worldGroup,opts={}) => {
  const object = new THREE.Mesh(geometry, typeof color === 'string' || typeof color === 'number' ? mat(color,opts) : color);
  object.position.set(x,y,z); object.castShadow=true; object.receiveShadow=true;
  if(parent) parent.add(object);
  return object;
};
const box = (w,h,d,color,x,y,z,parent=worldGroup) => mesh(new THREE.BoxGeometry(w,h,d),color,x,y,z,parent);
const sphere = (r,color,x,y,z,parent=worldGroup,detail=0) => mesh(new THREE.IcosahedronGeometry(r,detail),color,x,y,z,parent);
const cylinder = (rt,rb,h,color,x,y,z,parent=worldGroup,sides=7) => mesh(new THREE.CylinderGeometry(rt,rb,h,sides),color,x,y,z,parent);
const ground = (w,d,color,x,z,y=0) => {const m=mesh(new THREE.PlaneGeometry(w,d),color,x,y,z);m.rotation.x=-Math.PI/2;m.castShadow=false;return m;};
const textSprite = (text, color='#fff8e9', background=null, width=512, height=100) => {
  const c=document.createElement('canvas');c.width=width;c.height=height;
  const ctx=c.getContext('2d');
  if(background){ctx.fillStyle=background;ctx.beginPath();ctx.roundRect(1,1,width-2,height-2,20);ctx.fill();}
  ctx.font='bold 38px "Trebuchet MS", Arial';ctx.textAlign='center';ctx.textBaseline='middle';ctx.fillStyle=color;ctx.fillText(text,width/2,height/2,width-25);
  const tex=new THREE.CanvasTexture(c);tex.colorSpace=THREE.SRGBColorSpace;
  const sprite=new THREE.Sprite(new THREE.SpriteMaterial({map:tex,transparent:true,depthWrite:false}));
  sprite.scale.set(4,height/width*4,1);return sprite;
};
const flatLabel = (text,color,background,width=512,height=100) => {
  const sprite=textSprite(text,color,background,width,height);
  const label=new THREE.Mesh(new THREE.PlaneGeometry(1,1),new THREE.MeshBasicMaterial({map:sprite.material.map,transparent:true,side:THREE.DoubleSide,depthWrite:false}));
  label.scale.copy(sprite.scale);sprite.material.dispose();return label;
};
function addTree(x,z,scale=1,kind=0) {
  const group=new THREE.Group();worldGroup.add(group);group.position.set(x,0,z);group.scale.setScalar(scale);
  const h=5+random()*3;
  cylinder(.24,.53,h,'#73503b',0,h/2,0,group,6);
  const root1=box(.24,.25,2,'#72513a',0,.12,.4,group);root1.rotation.y=.6;
  if(kind===1) {
    for(let i=0;i<3;i++) cylinder(0,2.4-i*.4,3.7-i*.35,['#3f6f4d','#4b7d53','#60885b'][i],0,h-2+i*1.5,0,group,7);
  }else{
    const branch=cylinder(.13,.23,3,'#74513b',.9,h*.72,0,group,5);branch.rotation.z=-.6;
    const colors=['#507344','#63854c','#799651','#426640','#8a9f57'];
    sphere(2.6,colors[Math.floor(random()*colors.length)],0,h+.25,0,group,1).scale.set(1,1.06,.91);
    sphere(2.2,colors[Math.floor(random()*colors.length)],1.5,h-.5,.45,group,0);
    sphere(1.8,'#759348',-1.4,h-.2,.1,group,0);
    sphere(1.3,'#92a65a',.2,h+1.8,.15,group,0);
  }
  return group;
}
function addRock(x,z,scale=1,parent=worldGroup) {
  const rock=sphere(scale,'#999b7e',x,.45*scale,z,parent,0);rock.scale.set(1,.7,.78);rock.rotation.set(.1,random()*6,.15);return rock;
}
function addGrass(x,z,color='#718e4b',scale=1) {
  const g=new THREE.Group();g.position.set(x,0,z);worldGroup.add(g);
  for(let i=0;i<3;i++){const m=mesh(new THREE.ConeGeometry(.15,.9,3),color,(i-1)*.18,.35,0,g);m.rotation.z=(i-1)*-.3;m.scale.setScalar(scale);}
}
function addMushroom(x,z,scale=1) {
  cylinder(.07,.095,.35*scale,'#ede0b8',x,.18*scale,z);
  const cap=sphere(.28*scale,'#c8774a',x,.39*scale,z);cap.scale.y=.5;
}
function addPalm(x,z,scale=1) {
  const g=new THREE.Group();g.position.set(x,0,z);g.scale.setScalar(scale);worldGroup.add(g);
  const trunk=cylinder(.15,.35,5.5,'#ae8457',.3,2.7,0,g,6);trunk.rotation.z=-.1;
  for(let i=0;i<7;i++){const leaf=mesh(new THREE.ConeGeometry(.55,3.8,4),'#668f59',0,5.3,0,g);leaf.geometry.translate(0,1.5,0);leaf.rotation.set(.2,0,1.25);const pivot=new THREE.Group();g.remove(leaf);pivot.add(leaf);pivot.rotation.y=i*Math.PI/3.5;g.add(pivot);}
  sphere(.27,'#85603c',.55,5.2,0,g);sphere(.25,'#89633e',.1,5,0,g);
}
function addCloud(x,y,z,scale=1) {
  const g=new THREE.Group();g.position.set(x,y,z);g.scale.setScalar(scale);worldGroup.add(g);
  for(let i=0;i<5;i++){const m=sphere(3+random()*2,'#f2efdb',(i-2)*3,random(),random()*2,g,1);m.castShadow=false;m.scale.y=.5;}
}
function addBuilding(x,z,type,options={}) {
  const g=new THREE.Group();g.position.set(x,0,z);worldGroup.add(g);
  const desert=type==='desert'||type==='desert_building', bakery=type==='bakery', gym=type==='gym',shop=type.includes('shop');
  const wall=desert?'#cfaa79':bakery?'#e6d6b0':gym?'#839998':type==='kitchen_shop'?'#b87961':shop?'#b7b98d':'#aaa996';
  const w=options.width||(desert?8:bakery?9:12),h=options.height||(desert?4:5),d=options.depth||(desert?7:8);
  // Walk-in rooms: the stations and village trader live inside these walls.
  // The front doorway is genuinely open, with a lintel rather than a painted door.
  const door=3.2, doorHeight=3.2, wing=(w-door)/2;
  box(w,.12,d,desert?'#b69772':'#b8a382',0,.015,0,g);
  box(w,h,.24,wall,0,h/2,-d/2,g);
  for(const side of [-1,1]) {
    box(.24,h,d,wall,side*w/2,h/2,0,g);
    box(wing,h,.24,wall,side*(door/2+wing/2),h/2,d/2,g);
    box(.14,doorHeight,.36,desert?'#9b754f':'#927c5f',side*(door/2+.03),doorHeight/2,d/2,g);
  }
  box(door,h-doorHeight,.24,wall,0,doorHeight+(h-doorHeight)/2,d/2,g);
  box(door+.3,.16,.38,desert?'#9b754f':'#927c5f',0,doorHeight,d/2,g);
  box(w,.18,d,wall,0,h,0,g);
  for(const child of g.children)if(child.material===mat(wall))child.userData.paintTarget='walls';
  const rug=box(door+1,.025,d-1,desert?'#aa6650':bakery?'#ac8060':'#82917c',0,.09,0,g);
  for(const side of [-1,1]) {
    box(.35,.7,.26,'#936c3f',side*(w/2-.65),2.4,-d/2+.3,g);
    const lantern=sphere(.15,'#ffe2a0',side*(w/2-.65),2.5,-d/2+.52,g);
    lantern.material=mat('#ffe2a0',{emissive:'#efb559',emissiveIntensity:.6});
  }
  if(desert) {
    box(w+.5,.4,d+.5,'#b58a5f',0,h,0,g);
    for(let i=0;i<5;i++) box(.6,.7,.6,'#c39a6b',-w/2+i*w/4,h+.4,-d/2,g);
    const awning=box(4,.12,2.4,'#c67e4c',0,3,d/2+1.2,g);awning.rotation.x=.08;
    cylinder(.06,.07,3,'#8d7146',-1.9,1.5,d/2+2.2,g);cylinder(.06,.07,3,'#8d7146',1.9,1.5,d/2+2.2,g);
    sphere(.5,'#b97650',-3,.5,d/2+.5,g);cylinder(.4,.5,.7,'#b7774e',-3,.8,d/2+.5,g);
    for(const side of [-1,1]) box(1,1.15,.12,'#7a8066',side*2.6,2.4,d/2+.05,g);
  }else{
    const roof=cylinder(0,w*.8,2,'#79735c',0,h+.85,0,g,4);roof.rotation.y=Math.PI/4;roof.scale.z=d/w;
    for(const side of [-1,1]){box(2.1,1.8,.14,'#6f9990',side*(w/2-1.8),2.5,d/2+.08,g);box(.09,1.8,.2,'#eddfbb',side*(w/2-1.8),2.5,d/2+.16,g);}
    if(bakery){const a=box(7,.12,2.6,'#df996a',0,3.7,d/2+1.2,g);a.rotation.x=.12;for(let i=0;i<7;i++)box(.5,.12,2.65,'#f6e6bd',-3+i,3.73,d/2+1.2,g);}
    if(bakery||gym||shop){const sign=flatLabel(options.label||(bakery?'BAKERY PLOT':gym?'CRUMB CITY GYM':type==='kitchen_shop'?'KETTLE & COLOUR':'THE CORNER PANTRY'),bakery?'#74482d':'#354f43','#f3e6c5',512,90);sign.position.set(0,h-.65,d/2+.17);sign.scale.set(Math.min(w-1,9),1.1,1);g.add(sign);g.userData.sign=sign;}
    if(bakery){const oven=box(2.2,1.6,1.2,'#7f786b',-2.6,.85,-2.5,g);oven.userData.paintTarget='oven';box(1.4,.75,.08,'#3d443b',-2.6,.8,-1.85,g);cylinder(.2,.2,2.6,'#746b5b',-2.6,2.9,-2.5,g);box(2,.14,.8,'#8f7554',2.5,1.3,-2.5,g);}
    if(gym){for(const side of [-1,1]){box(1.2,.25,2.7,'#546a60',side*3,.5,-2,g);box(.09,1,.1,'#949989',side*3-.5,.9,-3,g);box(.09,1,.1,'#949989',side*3+.5,.9,-3,g);}box(4,.2,.85,'#bd9b70',0,.55,-2.8,g);}
    if(gym){
      for(const side of [-1,1]){box(2.2,2.35,.04,'#486c72',side*(w/2-1.6),1.75,d/2+.14,g);box(1.25,.075,.12,'#ddd9c0',side*(w/2-1.6),1.65,d/2+.21,g);for(const end of [-1,1])cylinder(.3,.3,.14,'#253c40',side*(w/2-1.6)+end*.51,1.65,d/2+.25,g,12).rotation.z=Math.PI/2;}
    }
    if(shop){box(w+.6,.22,1.8,type==='kitchen_shop'?'#7d9c88':'#d6ad67',0,3.45,d/2+.75,g);for(const side of [-1,1]){box(2.1,1.8,.05,'#658c8d',side*(w/2-1.6),2,d/2+.17,g);box(2,.15,.8,'#926a4c',side*(w/2-1.6),1,d/2+.5,g);}if(type==='kitchen_shop')for(let i=0;i<3;i++)cylinder(.22,.22,.35,['#c86656','#85a085','#d9b765'][i],-w/2+1+i*.5,1.28,d/2+.5,g);}
  }
  return g;
}
function coastAt(x) {
  const points=state.layout?.coast;
  if(points?.length){for(let i=1;i<points.length;i++)if(x<=points[i].x){const a=points[i-1],b=points[i];return a.z+(b.z-a.z)*clamp((x-a.x)/(b.x-a.x),0,1);}return points.at(-1).z;}
  return 32+11*Math.sin((x+240)*.021)+5*Math.sin(x*.057);
}
const worldBounds = () => state.layout||{minX:-240,maxX:240,minZ:-260,maxZ:200};
const scenery = new Map();
function createEnvironment(layout=null) {
  if(layout)state.layout=layout;
  for(const object of [...worldGroup.children]){worldGroup.remove(object);disposeObject(object);}scenery.clear();
  const bounds=worldBounds(),width=bounds.maxX-bounds.minX,depth=bounds.maxZ-bounds.minZ;
  const geometry=new THREE.PlaneGeometry(width,depth,120,115);geometry.rotateX(-Math.PI/2);
  const positions=geometry.attributes.position,colors=[];
  const forest=new THREE.Color('#82995f'),sand=new THREE.Color('#dfc391'),desert=new THREE.Color('#d4ae77'),city=new THREE.Color('#a6b091'),shallow=new THREE.Color('#9bb9ab'),deep=new THREE.Color('#467f89');
  for(let i=0;i<positions.count;i++){
    const x=positions.getX(i),z=positions.getZ(i)+(bounds.minZ+bounds.maxZ)/2,c=forest.clone();positions.setZ(i,z);
    if(x<0){const shore=coastAt(x),t=clamp((z-(shore-23))/17,0,1);c.lerp(sand,t);if(z>shore){const d=clamp((z-shore)/36,0,1);positions.setY(i,-.4-d*4.5);c.copy(shallow).lerp(deep,d);}else positions.setY(i,0);}
    else{c.copy(z<0?desert:city);if(z>-6&&z<6)c.copy(desert).lerp(city,(z+6)/12);if(x<10&&z<0)c.copy(forest).lerp(desert,x/10);}
    const variation=Math.sin(x*.17)*Math.cos(z*.2)*.018;c.offsetHSL(0,0,variation);colors.push(c.r,c.g,c.b);
  }
  geometry.setAttribute('color',new THREE.Float32BufferAttribute(colors,3));geometry.computeVertexNormals();
  const land=new THREE.Mesh(geometry,new THREE.MeshStandardMaterial({vertexColors:true,roughness:1,flatShading:true}));land.receiveShadow=true;worldGroup.add(land);
  const verts=[],indices=[],waterColors=[];
  for(let i=0;i<=80;i++){const x=bounds.minX+(0-bounds.minX)*i/80,coast=coastAt(x);for(let j=0;j<=52;j++){const t=j/52,z=coast+(bounds.maxZ-coast)*t;verts.push(x,.035,z);const c=new THREE.Color('#82c7c1').lerp(new THREE.Color('#377f94'),clamp(t*2,0,1));waterColors.push(c.r,c.g,c.b);if(i<80&&j<52){const n=i*53+j;indices.push(n,n+1,n+53,n+1,n+54,n+53);}}}
  const waterGeometry=new THREE.BufferGeometry();waterGeometry.setAttribute('position',new THREE.Float32BufferAttribute(verts,3));waterGeometry.setAttribute('color',new THREE.Float32BufferAttribute(waterColors,3));waterGeometry.setIndex(indices);waterGeometry.computeVertexNormals();
  water=new THREE.Mesh(waterGeometry,new THREE.MeshStandardMaterial({vertexColors:true,roughness:.3,metalness:.12,transparent:true,opacity:.85,side:THREE.DoubleSide,flatShading:true}));water.receiveShadow=true;worldGroup.add(water);waterPositions=waterGeometry.attributes.position.array.slice();
  const foamPoints=[];for(let i=0;i<=100;i++){const x=bounds.minX+(0-bounds.minX)*i/100;foamPoints.push(new THREE.Vector3(x,.1,coastAt(x)));}
  const foam=new THREE.Line(new THREE.BufferGeometry().setFromPoints(foamPoints),new THREE.LineBasicMaterial({color:'#ecedd1',transparent:true,opacity:.65}));worldGroup.add(foam);
  for(const p of state.layout?.paths||[]){ground(p.width,p.depth,p.z>=0&&p.x>=0?'#8c9187':'#b7ac80',p.x,p.z,.022);if(p.x>=0&&p.z>=0){const horizontal=p.width>p.depth;for(let i=-.5;i<.5;i+=.1)ground(horizontal?3:.15,horizontal?.15:3,'#e1d8af',p.x+(horizontal?i*p.width:0),p.z+(horizontal?0:i*p.depth),.029);}}
  for(const p of state.layout?.props||[]){
    let o;const s=p.scale||1;
    if(p.kind==='tree'||p.kind==='pine')o=addTree(p.x,p.z,s,p.kind==='pine'?1:0);
    else if(p.kind==='palm'){addPalm(p.x,p.z,s);o=worldGroup.children.at(-1);}
    else if(p.kind==='rock'||p.kind==='beach_rock'){o=addRock(p.x,p.z,s);if(p.kind==='beach_rock')o.material=mat('#cbb68e');}
    else if(p.kind==='bush'){o=sphere(s,'#6f8950',p.x,s*.48,p.z);o.scale.set(1,.7,.9);}
    else if(p.kind==='cactus'){o=new THREE.Group();worldGroup.add(o);o.position.set(p.x,0,p.z);buildCactus(o,s);}
    else o=addBuilding(p.x,p.z,p.kind,p);
    if(o){o.userData.prop=p;scenery.set(p.id,o);}
  }
  for(const zone of state.layout?.zones||[]){
    const sign=new THREE.Group();worldGroup.add(sign);sign.position.set(zone.x-zone.width/2+2,0,zone.z+zone.depth/2+1);
    cylinder(.075,.09,1.6,'#8a6a47',0,.8,0,sign);box(2.6,.7,.12,'#ab8955',0,1.4,0,sign);
    const label=flatLabel(zone.name,'#fff0d0');label.position.set(0,1.43,.08);label.scale.set(2.5,.48,1);sign.add(label);
  }
  const flowers=rng(2746);
  for(let i=0;i<520;i++){const x=-235+flowers()*228,z=-253+flowers()*278;if(z>coastAt(x)-22)continue;const blocked=(state.layout?.zones||[]).some(p=>Math.abs(x-p.x)<p.width/2&&Math.abs(z-p.z)<p.depth/2);if(!blocked){if(i%6===0)addMushroom(x,z,.6+flowers());else addGrass(x,z,'#718e4b',.5+flowers());}}
  for(let i=0;i<22;i++)addCloud(-230+random()*470,40+random()*30,-240+random()*450,1+random());
  const sun=sphere(6,'#fff0c4',-80,70,-245);sun.material=new THREE.MeshBasicMaterial({color:'#ffeebd'});sun.castShadow=false;
}
function buildCactus(g,scale=1){
  cylinder(.25,.3,2.3,'#7b9b61',0,1.15,0,g,6);sphere(.25,'#86a86d',0,2.28,0,g);
  const arm=cylinder(.14,.18,.8,'#77975b',.43,1.1,0,g,6);arm.rotation.z=Math.PI/2;
  cylinder(.15,.18,.9,'#89a36c',.77,1.5,0,g,6);sphere(.16,'#91af74',.77,1.95,0,g);g.scale.setScalar(scale);
}
function makeCookie(kind='sugar',scale=1) {
  const g=new THREE.Group();
  const colors={sugar:'#e6b776',berry_cookie:'#dbaa78',nut_cookie:'#be8b4d',cactus_cookie:'#c1b878',sun_cookie:'#efbd5b',salt_cookie:'#c8c7a4',protein_cookie:'#cfa685',cake:'#d5a36d'};
  if(kind==='cake'){
    box(.65,.2,.5,'#9b643d',0,0,0,g);box(.67,.11,.52,'#f5ddb0',0,.14,0,g);sphere(.08,'#b25556',0,.23,0,g);
  }else{
    cylinder(.34,.36,.105,colors[kind]||colors.sugar,0,0,0,g,12);
    const chip={berry_cookie:'#993d59',nut_cookie:'#785033',cactus_cookie:'#6d8d4b',sun_cookie:'#c58037',salt_cookie:'#fff4dc',protein_cookie:'#835c82'}[kind]||'#895e3b';
    const points=[[-.15,.14],[.14,.17],[.02,-.04],[-.18,-.12],[.19,-.16],[.25,.02]];
    points.forEach(([x,z],i)=>{const c=sphere(i%2?.043:.057,chip,x,.057,z,g);c.scale.y=.55;});
    for(const [x,z] of [[-.04,.25],[-.26,.04],[.06,-.25]])sphere(.014,'#f5d196',x,.057,z,g);
  }
  g.scale.setScalar(scale);return g;
}
function makePerson(name='',color='#df9d60',peace=false,avatar={}) {
  const g=new THREE.Group();
  const skin=skinTones[avatar.skin]||'#deb889',shirt=avatarColor(avatar.shirtColor,color),pants=avatarColor(avatar.pantsColor,'#596955');
  cylinder(.22,.31,.7,peace?'#99a46c':shirt,0,1.05,0,g,6);
  sphere(.25,skin,0,1.68,0,g,1);sphere(.27,peace?'#7b6551':'#5d4b36',0,1.8,.045,g).scale.y=.5;
  if(avatar.shirt==='hoodie'){const hood=sphere(.29,shirt,0,1.43,.08,g);hood.scale.z=.65;box(.25,.14,.035,pants,0,1,-.26,g);}
  if(avatar.shirt==='striped')for(let i=0;i<3;i++)box(.44,.045,.035,'#f0e6cf',0,.88+i*.16,-.255,g);
  for(const side of [-1,1]){
    box(.18,.65,.2,pants,side*.16,.38,0,g);
    if(avatar.pants==='shorts')box(.17,.28,.19,skin,side*.16,.24,-.005,g);
    const arm=cylinder(.08,.1,.66,skin,side*.36,1.05,0,g,6);arm.rotation.z=-side*.15;
    if(avatar.shirt!=='tank')cylinder(.115,.1,avatar.shirt==='hoodie'?.58:.25,shirt,side*.34,avatar.shirt==='hoodie'?1.09:1.26,0,g,6);
    box(.21,.14,.36,'#685846',side*.16,.1,-.07,g);
  }
  const held=makeCookie('sugar',.45);held.position.set(.37,.97,-.2);g.add(held);
  if(peace){cylinder(.5,.5,.06,'#dcb782',0,1.85,0,g,10);cylinder(.23,.3,.24,'#d5ad78',0,1.95,0,g,8);box(.4,.12,.08,'#cf8b70',0,1.1,-.23,g);}
  else if(avatar.hat&&avatar.hat!=='none'){
    if(avatar.hat==='chef'){cylinder(.25,.25,.25,'#f6edda',0,1.98,0,g);for(const x of [-.18,0,.18])sphere(.2,'#fff7e8',x,2.16,0,g);}
    else if(avatar.hat==='builder'){cylinder(.32,.32,.04,'#ddb148',0,1.9,0,g);sphere(.29,'#e7bd53',0,1.94,0,g).scale.y=.6;box(.07,.06,.47,'#f4ce76',0,2.1,0,g);}
    else if(avatar.hat==='champion'){cylinder(.29,.29,.16,'#daba60',0,1.98,0,g);for(let i=0;i<5;i++){const a=i*Math.PI*2/5;mesh(new THREE.ConeGeometry(.085,.24,4),'#efd27a',Math.cos(a)*.23,2.12,Math.sin(a)*.23,g);}}
    else if(avatar.hat==='recycler'){cylinder(.39,.39,.04,'#688f62',0,1.91,0,g);cylinder(.21,.29,.23,'#7fa373',0,2.01,0,g);}
    else{cylinder(.27,.3,.16,'#668faa',0,1.95,0,g);box(.35,.045,.21,'#7fa9bb',0,1.91,-.28,g);}
  }
  if(name){const label=textSprite(name,'#fff6db','#233f2cca',512,96);label.position.y=2.3;label.scale.set(2.2,.42,1);g.add(label);}
  return g;
}
function homeObject(home){
  const g=new THREE.Group();g.position.set(home.x,0,home.z);const cabin=home.kind==='cabin',w=cabin?5.6:4.6,d=cabin?4.8:4.4;
  box(w,.12,d,'#ac8c60',0,.04,0,g);
  if(cabin){
    box(w,3.6,.8,'#a67b50',0,1.8,d/2-.4,g);
    for(const side of [-1,1]){box(.8,3.6,d,'#a67b50',side*(w/2-.4),1.8,0,g);box(1.8,3.6,.8,'#a67b50',side*1.9,1.8,-d/2+.4,g);}
    box(2,.65,.8,'#a67b50',0,3.275,-d/2+.4,g);
    for(let y=.4;y<3.5;y+=.45)box(w+.08,.045,.83,'#805e40',0,y,d/2-.4,g);
    const roof=cylinder(0,w*.78,2,'#657d63',0,4.1,0,g,4);roof.rotation.y=Math.PI/4;roof.scale.z=d/w;
  }else{
    const color=home.safe?'#8d9c71':'#c2a175',roof=new THREE.BufferGeometry();
    roof.setAttribute('position',new THREE.Float32BufferAttribute([-w/2,0,-d/2,0,3,-d/2,0,3,d/2,-w/2,0,d/2,w/2,0,-d/2,w/2,0,d/2,0,3,d/2,0,3,-d/2],3));roof.setIndex([0,1,2,0,2,3,4,5,6,4,6,7]);roof.computeVertexNormals();mesh(roof,color,0,0,0,g,{side:THREE.DoubleSide});
    box(w,.9,2.65,color,0,.5,.8,g);box(w,.6,.15,color,0,1.15,d/2,g);
    for(const z of [-d/2,d/2])cylinder(.045,.055,3.1,'#795c3e',0,1.55,z,g,6);
  }
  const hearth=nodeObject({kind:'fire',x:0,z:0});hearth.scale.setScalar(.65);hearth.position.set(0,.05,cabin?.5:-1.25);g.add(hearth);
  const sign=new THREE.Group();sign.position.set(w/2+.55,0,-d/2-.5);g.add(sign);cylinder(.065,.08,1.6,'#856440',0,.8,0,sign);box(2.8,.85,.12,'#a07f52',0,1.5,0,sign);
  const name=flatLabel(home.callName||'A wandering baker','#fff0ce',null,512,80);name.scale.set(2.55,.4,1);name.position.set(0,1.65,-.075);name.rotation.y=Math.PI;sign.add(name);
  const account=flatLabel('@'+(home.username||'baker'),'#e6d8b6',null,512,75);account.scale.set(2.5,.34,1);account.position.set(0,1.29,-.075);account.rotation.y=Math.PI;sign.add(account);
  const safety=new THREE.Mesh(new THREE.RingGeometry(7.85,8,80),new THREE.MeshBasicMaterial({color:'#badfa4',transparent:true,opacity:.28,side:THREE.DoubleSide,depthWrite:false}));safety.rotation.x=-Math.PI/2;safety.position.y=.06;safety.name='home-safe-ring';safety.visible=home.safe&&home.owner===state.snapshot?.me.id;g.add(safety);
  g.userData.signature=JSON.stringify(home);return g;
}
function updateBakeryAppearance(n){
  const building=scenery.get(n.id);if(!building)return;
  const own=n.owner===state.snapshot?.me.id,level=n.bakeryLevel||(own?state.snapshot.me.bakeryLevel:0)||0,equipment=n.equipment||(own?state.snapshot.me.equipment:{}),paint=n.paint||{};
  const signature=JSON.stringify([n.owner,n.callName,n.username,paint,equipment,level]);if(building.userData.bakerySignature===signature)return;building.userData.bakerySignature=signature;
  if(building.userData.sign){building.remove(building.userData.sign);disposeObject(building.userData.sign);}
  const sign=flatLabel(n.owner?(n.callName||'A baker')+"'s BAKERY":'BAKERY · 250 COINS','#74482d','#f3e6c5',768,100);sign.position.set(0,4.35,4.67);sign.scale.set(8.5,1.1,1);building.add(sign);building.userData.sign=sign;
  const old=building.getObjectByName('bakery-custom');if(old){building.remove(old);disposeObject(old);}
  const custom=new THREE.Group();custom.name='bakery-custom';building.add(custom);
  if(n.owner){const username=flatLabel('@'+(n.username||'baker'),'#605c40','#eaddb7',512,75);username.position.set(0,3.58,4.69);username.scale.set(3.8,.5,1);custom.add(username);}
  if(equipment?.mixer){const stand=box(.7,.7,.65,avatarColor(paint.mixer,'#7f9b90'),2.5,1.68,-2.5,custom);stand.userData.paintTarget='mixer';sphere(.34,'#c7ccc0',2.5,1.4,-2.28,custom).scale.y=.6;}
  if(equipment?.display){box(2.7,1.1,.7,avatarColor(paint.display,'#b99d75'),2.4,.6,2,custom);box(2.8,.045,.8,'#d1e7d4',2.4,1.6,2,custom);for(let i=0;i<3;i++){const cookie=makeCookie(i===2?'cake':'sugar',.8);cookie.position.set(1.6+i*.75,1.2,2);custom.add(cookie);}}
  if(equipment?.oven){box(1.85,.15,1.15,'#d2cbb5',-2.6,1.74,-2.5,custom);for(let i=0;i<3;i++)sphere(.065,'#e6c98d',-3.15+i*.5,1.37,-1.83,custom);}
  for(let i=0;i<Math.min(level,8);i++){const loaf=makeCookie('sugar',.55);loaf.position.set(-3.6+(i%4)*.32,1.1+Math.floor(i/4)*.12,2.65);custom.add(loaf);}
  if(level){const badge=flatLabel('OVEN LEVEL '+level,'#e3c984','#4c6b58',384,70);badge.position.set(-3.3,2.15,4.69);badge.scale.set(2,.4,1);custom.add(badge);}
  building.traverse(o=>{if(o.userData.paintTarget){const target=o.userData.paintTarget,fallback=target==='walls'?'#e6d6b0':target==='oven'?'#7f786b':'#b99d75';o.material=mat(avatarColor(paint[target],fallback));}});
}
function nodeObject(n) {
  const g=new THREE.Group();
  switch(n.kind){
    case 'wood': for(let i=0;i<3;i++){const m=cylinder(.19,.22,1.4,'#91673f',(i-1)*.37,.24,i%2*.15,g,7);m.rotation.z=Math.PI/2;sphere(.18,'#bb9659',.72,.24,i%2*.15,g).scale.x=.15;}break;
    case 'stick':for(let i=0;i<4;i++){const m=cylinder(.035,.06,1.25,'#927a48',(i-2)*.12,.12,0,g,5);m.rotation.set(.2*i,.3*i,Math.PI/2-.1*i);}break;
    case 'stone':{const a=addRock(-.25,0,.35,g),b=addRock(.3,.1,.28,g);if(n.x<0&&n.z>coastAt(n.x)-18)a.material=b.material=mat('#cab590');break;}
    case 'dough':{const patch=cylinder(1.1,1.1,.025,'#d6c994',0,.027,0,g,12);for(let i=0;i<5;i++){const m=box(.65,.03,.055,'#b6a67b',(i-2)*.19,.057,(i%2-.5)*.35,g);m.rotation.y=-.7;}sphere(.1,'#eddeb0',.3,.1,.15,g);break;}
    case 'berry':sphere(.65,'#647f48',0,.5,0,g,1);sphere(.5,'#79924e',.4,.45,.1,g);for(let i=0;i<10;i++){const a=i*2.4;sphere(.105,'#b7504f',Math.cos(a)*.53,.65+(i%3)*.13,Math.sin(a)*.5,g);}break;
    case 'nut':sphere(.55,'#829355',0,.42,0,g);for(let i=0;i<7;i++)sphere(.15,'#9c7047',Math.cos(i)*.5,.13,Math.sin(i)*.5,g);break;
    case 'cactus':buildCactus(g);break;
    case 'sea_salt':for(let i=0;i<6;i++){const m=box(.15,.17,.17,'#f5ecce',(i%3-1)*.2,.12,Math.floor(i/3)*.2,g);m.rotation.y=i*.3;}break;
    case 'shell':{const m=mesh(new THREE.ConeGeometry(.38,.23,8),'#e5bb9c',0,.12,0,g);m.rotation.z=.3;break;}
    case 'trash':{const b=cylinder(.11,.11,.52,'#8cb7b0',-.16,.13,0,g,8);b.rotation.z=1.2;box(.27,.19,.3,'#cd9b81',.22,.13,.12,g);const ring=mesh(new THREE.TorusGeometry(.18,.025,4,12),'#b7c9b3',.08,.11,-.18,g);ring.rotation.x=Math.PI/2;break;}
    case 'chest':box(1.2,.7,.75,'#9b7546',0,.35,0,g);box(1.25,.18,.8,'#b88b50',0,.75,0,g);for(const side of [-1,1])box(.11,.9,.82,'#d4b567',side*.4,.44,0,g);box(.19,.2,.08,'#ded097',0,.55,.43,g);break;
    case 'fire':{for(let i=0;i<8;i++){const a=i*Math.PI/4;addRock(Math.cos(a)*.7,Math.sin(a)*.7,.25,g);}for(let i=0;i<3;i++){const m=cylinder(.1,.14,1.1,'#665442',0,.19,0,g,6);m.rotation.z=Math.PI/2;m.rotation.y=i*2.1;}const flame=mesh(new THREE.ConeGeometry(.34,1.1,5),new THREE.MeshBasicMaterial({color:'#ffbb60'}),0,.55,0,g);flame.name='flame';const flame2=mesh(new THREE.ConeGeometry(.18,.75,5),new THREE.MeshBasicMaterial({color:'#ffedb1'}),0,.47,.08,g);flame2.name='flame2';const light=new THREE.PointLight('#ffae62',5,9,2);light.position.y=.8;g.add(light);break;}
    case 'peace':{g.add(makePerson('PEACE','#8eaa73',true));box(3.5,1,1.1,'#ad8656',0,.5,-1.8,g);for(const side of [-1,1])cylinder(.055,.075,3,'#a78655',side*1.8,1.5,-1.8,g,6);const roof=box(4,.11,2.4,'#dca476',0,2.9,-1.8,g);roof.rotation.z=.06;for(let i=0;i<5;i++)box(.4,.12,2.4,'#e9d5a3',-1.6+i*.8,2.94,-1.8,g);const sign=flatLabel('GOOD KARMA TRADING','#6a623c','#e8d8ae',512,100);sign.position.set(0,.7,-1.22);sign.scale.set(2.9,.58,1);g.add(sign);break;}
    case 'desert_trader':g.add(makePerson(n.id==='desert_trader_2'?'JUNIPER':n.id==='desert_trader_3'?'CLOVE':'SAFFRON','#ba7750'));break;
    case 'kitchen_shop':g.add(makePerson('THE WORKSHOP','#779b8b'));break;
    case 'general_shop':g.add(makePerson('CRUMB MARKET','#b39465'));break;
    case 'paint_shop':g.add(makePerson('PIGMENT & COLOUR','#927cad'));break;
    case 'outfit_shop':g.add(makePerson('THREAD & THIMBLE','#b67c83'));break;
    case 'bakery_plot':{const menu=flatLabel('OVEN & BAKERY','#714e33','#edddaf',384,80);menu.position.set(0,1.7,0);menu.scale.set(2.1,.5,1);g.add(menu);cylinder(.055,.07,1.5,'#8d724e',0,.75,0,g);break;}
    case 'gym':{const bar=cylinder(.07,.07,2,'#6d706b',0,.65,0,g,8);bar.rotation.z=Math.PI/2;for(const side of [-1,1]){const weight=cylinder(.4,.4,.25,'#536b63',side*.75,.65,0,g,10);weight.rotation.z=Math.PI/2;}break;}
    case 'vending':box(1.3,2.2,.8,'#81928a',0,1.1,0,g);box(.93,1.23,.1,'#4c7067',-.07,1.36,.46,g);for(let i=0;i<6;i++)cylinder(.075,.07,.24,['#e3c398','#b493bc','#d4d5a5'][i%3],-.33+(i%3)*.29,1.16+Math.floor(i/3)*.46,.55,g);box(.7,.17,.12,'#294d43',0,.37,.45,g);break;
    case 'land':cylinder(.05,.07,1.7,'#897343',0,.85,0,g,5);box(1.65,.8,.12,'#ebd8a8',0,1.45,0,g);{const sign=flatLabel('YOUR BAKERY HERE','#6c7850',null,512,120);sign.position.set(0,1.47,.08);sign.scale.set(1.55,.37,1);g.add(sign);}break;
    case 'bakery':box(2,.8,1,'#bb9162',0,.4,0,g);for(let i=0;i<3;i++){const c=makeCookie(i===2?'cake':'sugar',1);c.position.set((i-1)*.55,.9,0);g.add(c);}break;
    case 'dummy':cylinder(.09,.14,1.5,'#937546',0,.75,0,g,6);{const target=cylinder(.5,.5,.2,'#d5a65e',0,1.6,0,g,12);target.rotation.x=Math.PI/2;const ring=mesh(new THREE.TorusGeometry(.31,.025,4,16),'#a16343',0,1.6,.12,g);sphere(.075,'#a16343',0,1.6,.12,g);}break;
    default:sphere(.35,'#ccb477',0,.3,0,g);
  }
  g.position.set(n.x,0,n.z);g.userData.node=n;
  return g;
}
function makeAnimal(a){
  const g=new THREE.Group(),s=a.species;
  const size={whale:3.4,shark:1.5,dolphin:1.1,sea_lion:1.2,turtle:.65,fish:.5}[s]||.8;
  const color={whale:'#5d8796',shark:'#829593',dolphin:'#8fb5b2',sea_lion:'#a58e6d',turtle:'#83986b',fish:'#d0b87c'}[s]||'#81aaa5';
  if(s==='turtle'){
    const shell=sphere(.65,'#6f8354',0,.2,0,g,1);shell.scale.set(1,.55,1.15);sphere(.22,'#aaa76d',0,.12,-.75,g);for(const side of [-1,1])for(const z of [-.4,.4]){const flipper=sphere(.25,'#a0a474',side*.59,.08,z,g);flipper.scale.set(1,.2,.6);}{const ring=mesh(new THREE.TorusGeometry(.72,.025,4,14),'#b7cebb',0,.25,0,g);ring.name='rescue-plastic';ring.rotation.x=Math.PI/2;ring.visible=a.need==='trapped';}
  }else{
    const body=sphere(size,color,0,0,0,g,1);body.scale.set(.42,.36,1);
    const tail=mesh(new THREE.ConeGeometry(size*.42,size*.65,3),color,0,0,size*1.05,g);tail.rotation.x=Math.PI/2;tail.rotation.z=Math.PI/2;tail.scale.z=.2;
    for(const side of [-1,1]){const fin=mesh(new THREE.ConeGeometry(size*.25,size*.8,3),color,side*size*.42,-size*.08,0,g);fin.rotation.z=side*.9;fin.scale.z=.2;}
    if(s==='shark'||s==='dolphin'){const fin=mesh(new THREE.ConeGeometry(size*.24,size*.48,3),color,0,size*.35,size*.1,g);fin.scale.z=.5;}
    for(const side of [-1,1])sphere(size*.035,'#273b36',side*size*.24,size*.07,-size*.72,g);
    if(s==='sea_lion'){sphere(size*.4,color,0,size*.16,-size*.8,g);sphere(size*.09,'#3f5145',0,size*.17,-size*1.16,g);}
  }
  const wound=sphere(.16,'#b36b68',s==='turtle'?.35:size*.34,s==='turtle'?.45:size*.1,-size*.15,g);wound.name='animal-wound';wound.scale.set(.16,1,.8);wound.visible=a.need==='wounded';
  const label=textSprite(pretty(s)+(a.need?' · '+pretty(a.need):''),'#fff3dc','#294b43b0',512,90);label.position.y=size+.7;label.scale.set(2.8,.5,1);g.add(label);g.userData.label=label;
  return g;
}
function initializeRenderer(){
  try{
    renderer=new THREE.WebGLRenderer({canvas:$('world'),antialias:true,powerPreference:'high-performance'});
    renderer.setPixelRatio(Math.min(window.devicePixelRatio,1.6));renderer.setSize(innerWidth,innerHeight);
    renderer.shadowMap.enabled=true;renderer.shadowMap.type=THREE.PCFSoftShadowMap;
    renderer.toneMapping=THREE.ACESFilmicToneMapping;renderer.toneMappingExposure=1.22;
    scene=new THREE.Scene();scene.background=new THREE.Color('#b7cbb8');scene.fog=new THREE.Fog('#a9c1a5',75,235);
    camera=new THREE.PerspectiveCamera(68,innerWidth/innerHeight,.06,420);camera.rotation.order='YXZ';scene.add(camera);
    scene.add(new THREE.HemisphereLight('#f9efd3','#718b5b',2.15));
    const sun=new THREE.DirectionalLight('#ffe4ac',3.25);sun.position.set(-50,70,-30);sun.castShadow=true;sun.shadow.mapSize.set(2048,2048);sun.shadow.camera.left=-65;sun.shadow.camera.right=65;sun.shadow.camera.top=65;sun.shadow.camera.bottom=-65;sun.shadow.camera.far=160;sun.shadow.bias=-.0004;sun.shadow.normalBias=.055;scene.add(sun);sun.target.position.set(-22,0,-20);scene.add(sun.target);
    worldGroup=new THREE.Group();scene.add(worldGroup);scene.add(dynamicGroup);createEnvironment();
    hand=new THREE.Group();hand.position.set(.48,-.43,-.82);camera.add(hand);
    const palm=box(.17,.18,.29,'#d8ad7d',.1,-.09,.12,hand);palm.name='hand-skin';palm.rotation.z=-.1;const sleeve=cylinder(.115,.15,.53,'#688660',.15,-.21,.34,hand,6);sleeve.name='hand-sleeve';sleeve.rotation.x=1.03;const cookie=makeCookie('sugar',1);cookie.name='held-cookie';hand.add(cookie);
    hand.rotation.set(.55,-.25,-.24);hand.visible=false;
    miniContext=$('minimap').getContext('2d');
    camera.position.set(-27,3.4,-23);camera.lookAt(-42,2,-44);
    state.worldReady=true;$('loading').style.opacity='0';setTimeout(()=>$('loading').hidden=true,450);
    requestAnimationFrame(animate);
  }catch(error){console.error('World initialization failed',error);$('loading').hidden=true;$('render-error').hidden=false;}
}
async function api(path, data) {
  const response=await fetch(path,{method:data?'POST':'GET',credentials:'same-origin',headers:data?{'Content-Type':'application/json'}:{},body:data?JSON.stringify(data):undefined});
  let result;try{result=await response.json();}catch{throw new Error('The server could not answer. Please try again.');}
  if(!response.ok)throw new Error(result.error||result.message||'Something went wrong. Please try again.');
  return result;
}
function authMessage(message){$('auth-message').textContent=message;$('auth-message').hidden=!message;}
$('email-form').addEventListener('submit',async(event)=>{
  event.preventDefault();authMessage('');const button=$('email-submit');button.disabled=true;
  state.email=$('email').value.trim();
  try{const result=await api('/api/login',{email:state.email});$('email-stage').hidden=true;$('code-stage').hidden=false;$('code-destination').textContent='We sent a code to '+state.email+'.';$('development-code').hidden=!result.devCode;if(result.devCode){$('development-code').textContent='Local development: your code is '+result.devCode+'. Enter it below to continue.';$('login-code').value=result.devCode;$('code-destination').textContent='Local development login is ready.';}$('login-code').focus();}
  catch(error){authMessage(error.message);}finally{button.disabled=false;}
});
$('code-form').addEventListener('submit',async(event)=>{
  event.preventDefault();authMessage('');const button=$('code-submit');button.disabled=true;
  try{const result=await api('/api/verify',{email:state.email,code:$('login-code').value.trim()});enterGame(result.player);}
  catch(error){authMessage(error.message);}finally{button.disabled=false;}
});
$('change-email').addEventListener('click',()=>{$('email-stage').hidden=false;$('code-stage').hidden=true;authMessage('');$('email').focus();});
$('logout-button').addEventListener('click',async()=>{
  try{await api('/api/logout',{});state.player=null;state.connected=false;if(state.ws)state.ws.close();location.reload();}catch(error){toast(error.message,'error');}
});
function enterGame(player){
  state.player=player;$('landing').hidden=true;document.body.classList.add('playing');$('hud').hidden=false;$('player-name').textContent=player.callName||player.name;$('start-overlay').hidden=false;
  connect();
}
let reconnectTimer;
function connect(){
  if(!state.player||state.sessionReplaced)return;
  clearTimeout(reconnectTimer);
  const ws=new WebSocket((location.protocol==='https:'?'wss://':'ws://')+location.host+'/ws');state.ws=ws;
  ws.addEventListener('open',()=>{state.connected=true;$('disconnect-banner').hidden=true;$('connection-state').innerHTML='<i></i> CONNECTED';});
  ws.addEventListener('message',(event)=>{
    try{const message=JSON.parse(event.data);if(message.type==='snapshot')receiveSnapshot(message);else if(message.type==='error')toast(message.message||'That action is not available.','error');}catch(error){console.warn('Could not read game update',error);}
  });
  ws.addEventListener('close',(event)=>{
    if(state.ws!==ws||!state.player)return;
    state.connected=false;state.keys.clear();$('disconnect-banner').hidden=false;$('connection-state').textContent='RECONNECTING';
    if(event.code===4001||event.code===1008){
      state.sessionReplaced=true;if(document.pointerLockElement)document.exitPointerLock();
      const message=event.code===4001?'Account opened in another tab. Reload to reconnect.':'Your session ended. Reload to sign in again.';
      $('disconnect-banner').textContent=message;$('connection-state').textContent='DISCONNECTED';toast(message,'error');return;
    }
    reconnectTimer=setTimeout(connect,2000);
  });
  ws.addEventListener('error',()=>{state.connected=false;});
}
function send(message){if(state.ws&&state.ws.readyState===WebSocket.OPEN){state.ws.send(JSON.stringify(message));return true;}return false;}
function action(actionName,extra={}){
  if(!state.connected){toast(state.sessionReplaced?'Account opened in another tab. Reload to reconnect.':'Reconnect to the world to do that.','error');return;}
  send({type:'action',action:actionName,...extra});
}
function movement(){
  if(!state.locked||state.modal||!state.connected)return{x:0,z:0,sprint:false};
  const forward=(state.keys.has('KeyW')||state.keys.has('ArrowUp')?1:0)-(state.keys.has('KeyS')||state.keys.has('ArrowDown')?1:0);
  const side=(state.keys.has('KeyD')||state.keys.has('ArrowRight')?1:0)-(state.keys.has('KeyA')||state.keys.has('ArrowLeft')?1:0);
  const size=Math.hypot(forward,side)||1;
  return {x:(-Math.sin(state.yaw)*forward+Math.cos(state.yaw)*side)/size,z:(-Math.cos(state.yaw)*forward-Math.sin(state.yaw)*side)/size,sprint:state.keys.has('ShiftLeft')||state.keys.has('ShiftRight')};
}
setInterval(()=>{if(state.connected){const move=movement();send({type:'input',...move,yaw:state.yaw,pitch:state.pitch});}},50);
function receiveSnapshot(snapshot){
  if(!snapshot.me)return;
  const first=!state.snapshot,old=state.snapshot?.me;
  if(snapshot.layout&&state.layout?.version!==snapshot.layout.version)createEnvironment(snapshot.layout);
  else if(snapshot.layout)state.layout=snapshot.layout;
  state.snapshot=snapshot;state.serverRecipes=snapshot.recipes||[];state.lastSnapshot=performance.now();
  if(first){state.localPosition.set(snapshot.me.x,(snapshot.me.y||0)+1.7,snapshot.me.z);state.yaw=snapshot.me.yaw||0;state.pitch=snapshot.me.pitch||0;state.selected=snapshot.me.selected||'sugar';updateHeldCookie();}
  if(old&&snapshot.me.health<old.health-1){$('damage-overlay').style.opacity='.5';setTimeout(()=>$('damage-overlay').style.opacity='0',240);}
  if(old&&snapshot.me.deaths>old.deaths){toast('You were crumbled. Your safe slots and hotbar are still yours. Back to the Wildwood!','error');state.localPosition.set(snapshot.me.x,(snapshot.me.y||0)+1.7,snapshot.me.z);}
  state.selectedSlot=snapshot.me.selectedSlot||0;
  if(snapshot.me.selected!==state.selected){state.selected=snapshot.me.selected||'';updateHeldCookie();}
  const avatarSignature=JSON.stringify(snapshot.me.avatar||{});if(state.avatarSignature!==avatarSignature){state.avatarSignature=avatarSignature;const a=snapshot.me.avatar||{};hand.getObjectByName('hand-skin').material=mat(skinTones[a.skin]||skinTones[1]);hand.getObjectByName('hand-sleeve').material=mat(a.shirt==='tank'?(skinTones[a.skin]||skinTones[1]):avatarColor(a.shirtColor,'#688660'));}
  syncObjects(snapshot.nodes||[],state.nodes,nodeObject,(o,n)=>{o.visible=n.available!==false||n.kind==='cactus';o.userData.node=n;if(n.kind==='bakery_plot')updateBakeryAppearance(n);});
  syncObjects(snapshot.players||[],state.players,()=>new THREE.Group(),(o,p)=>{const signature=JSON.stringify([p.callName,p.name,p.avatar]);if(o.userData.signature!==signature){for(const child of [...o.children]){o.remove(child);disposeObject(child);}o.add(makePerson(p.callName||p.name,'#d19b69',false,p.avatar||{}));o.userData.signature=signature;}o.userData.target=new THREE.Vector3(p.x,p.y||0,p.z);o.userData.yaw=p.yaw;if(!o.userData.initialized){o.position.copy(o.userData.target);o.userData.initialized=true;}});
  syncObjects((snapshot.animals||[]).filter((a)=>a.health>0),state.animals,makeAnimal,(o,a)=>{o.userData.target=new THREE.Vector3(a.x,a.y||0,a.z);o.scale.setScalar(a.scale||1);if(!o.userData.initialized){o.position.copy(o.userData.target);o.userData.initialized=true;}o.userData.animal=a;});
  syncObjects((snapshot.homes||[]).map(h=>({...h,id:h.owner})),state.homes,()=>new THREE.Group(),(o,h)=>{const signature=JSON.stringify([h.x,h.z,h.kind,h.safe,h.username,h.callName]);if(o.userData.signature!==signature){for(const child of [...o.children]){o.remove(child);disposeObject(child);}o.add(homeObject(h));o.userData.signature=signature;}});
  syncObjects(snapshot.projectiles||[],state.projectiles,(p)=>{const g=makeCookie(p.item,.65);g.position.set(p.x,p.y,p.z);return g;},(o,p)=>{o.userData.target=new THREE.Vector3(p.x,p.y,p.z);o.userData.velocity=new THREE.Vector3(p.vx,p.vy,p.vz);});
  for(const event of snapshot.events||[]){
    if(state.events.has(event.id))continue;state.events.add(event.id);
    if(!first||snapshot.time-event.time<2){addFeed(event.text,event.kind);if((event.kind==='hit'||event.kind==='defeat')&&event.actor===state.player.id){const reward=event.reward>0?' <span class="hit-reward">'+escapeHTML(event.hitZone==='head'?'HEADSHOT':event.hitZone==='hand/foot'?'HAND / FOOT':'BODY')+' +'+event.reward+' coins</span>':'';$('hit-marker').innerHTML='×'+reward;$('hit-marker').hidden=false;clearTimeout(state.hitTimer);state.hitTimer=setTimeout(()=>$('hit-marker').hidden=true,event.reward?900:200);}}
  }
  if(state.events.size>300)state.events=new Set((snapshot.events||[]).map((e)=>e.id));
  const signature=JSON.stringify([snapshot.me.safeSlots,snapshot.me.bagSlots,snapshot.me.hotbar,snapshot.me.recovery,snapshot.me.discovered,snapshot.me.coins,snapshot.me.levels,snapshot.me.bakery,snapshot.me.bakeryLevel,snapshot.me.home,snapshot.me.avatar,snapshot.me.callName,snapshot.me.hats,snapshot.me.equipment,snapshot.me.paint,snapshot.me.canBuildHome,snapshot.me.homeSafeReason]);
  if(signature!==state.inventorySignature){state.inventorySignature=signature;updateHUD();if(state.modal&&state.modal!=='map'&&state.modal!=='help')renderModal();}
  if(first)updateHUD();
}
function syncObjects(items,collection,create,update){
  const alive=new Set();
  for(const item of items){alive.add(item.id);let object=collection.get(item.id);if(!object){object=create(item);collection.set(item.id,object);dynamicGroup.add(object);}update(object,item);}
  for(const [id,object] of collection){if(!alive.has(id)){dynamicGroup.remove(object);disposeObject(object);collection.delete(id);}}
}
function disposeObject(object){object.traverse((child)=>{if(child.isMesh)child.geometry?.dispose();if(child.isSprite){child.material?.map?.dispose();child.material?.dispose();}});}
function updateHeldCookie(){
  if(!hand)return;const old=hand.getObjectByName('held-cookie');if(old){hand.remove(old);disposeObject(old);}if(cookieNames[state.selected]){const held=makeCookie(state.selected,1);held.name='held-cookie';hand.add(held);}
}
function cookieIcon(kind){return '<span class="cookie-icon '+escapeHTML(kind)+'" aria-hidden="true"></span>';}
function itemIcon(kind){return cookieNames[kind]?cookieIcon(kind):'<span class="material-icon" aria-hidden="true">'+(materialEmoji[kind]||'◇')+'</span>';}
function inventory(){return state.snapshot?.me.inventory||{};}
function itemCount(key){return inventory()[key]||0;}
function itemName(key){return cookieNames[key]?cookieName(key):pretty(key);}
function areaFor(x,z){return x<0?(z>coastAt(x)-18?'ocean':'forest'):(z<0?'desert':'city');}
const areaNames={forest:'THE WILDWOOD',desert:'SUNBAKED SANDS',ocean:'THE BLUE',city:'CRUMB CITY'};
let zoneTimer;
function updateZone(me){
  const zone=me.zone||areaNames[areaFor(me.x,me.z)];
  if(state.zone===zone)return;state.zone=zone;
  if(!state.hasPlayed)return;
  const el=$('zone-notification');el.querySelector('strong').textContent=zone;el.hidden=false;clearTimeout(zoneTimer);zoneTimer=setTimeout(()=>el.hidden=true,3300);
}
function updateHUD(){
  const me=state.snapshot?.me;if(!me)return;
  updateZone(me);
  $('coin-count').textContent=me.coins||0;$('player-name').textContent=me.callName||me.name;
  $('health-value').textContent=Math.ceil(me.health);$('health-bar').style.width=clamp(me.health/me.maxHealth*100,0,100)+'%';
  $('stamina-value').textContent=Math.ceil(me.stamina);$('stamina-bar').style.width=clamp(me.stamina/me.maxStamina*100,0,100)+'%';
  $('area-name').textContent=areaNames[areaFor(me.x,me.z)];$('position-label').textContent=Math.round(me.x)+' · '+Math.round(me.z);
  const angle=((state.yaw*180/Math.PI)%360+360)%360;$('compass-direction').textContent=['N','NW','W','SW','S','SE','E','NE'][Math.round(angle/45)%8];
  const slots=Array.from({length:5},(_,i)=>me.hotbar?.[i]||{item:'',count:0});state.hotkeys=slots.map(s=>s.item);
  const hotbarSignature=JSON.stringify([slots,state.selectedSlot,me.discovered]);
  if(hotbarSignature!==state.hotbarSignature){
    state.hotbarSignature=hotbarSignature;
    $('hotbar').innerHTML=slots.map((slot,i)=>'<button class="hotbar-slot '+(i===state.selectedSlot?'selected ':'')+(!slot.item?'empty':'')+'" data-slot="'+i+'" title="'+escapeHTML(slot.item?itemName(slot.item):'Empty hotbar slot')+' · '+(i+1)+'"><span class="slot-number">'+(i+1)+'</span>'+(slot.item?itemIcon(slot.item)+'<span class="slot-count">'+slot.count+'</span>':'<span class="empty-hotbar">+</span>')+'<span class="slot-safe" aria-label="Kept on death">◆</span></button>').join('');
  }
  const selected=slots[state.selectedSlot];
  $('selected-cookie-label').innerHTML=selected.item?escapeHTML(itemName(selected.item).toUpperCase())+' <span>'+escapeHTML(cookieNames[selected.item]?cookieNote(selected.item):'Ready in your hotbar')+'</span>':'EMPTY SLOT <span>Move an item here from your inventory</span>';
  $('buffs').innerHTML=Object.entries(me.buffs||{}).filter(([,seconds])=>seconds>0).map(([key,seconds])=>'<div class="buff"><span>✦</span><strong>'+escapeHTML(buffNames[key]||pretty(key).toUpperCase())+'</strong><span>'+Math.ceil(seconds)+'s</span></div>').join('');
  const need=Math.min(itemCount('wood'),2)+Math.min(itemCount('stick'),2)+Math.min(itemCount('stone'),3);
  if(me.bakery){$('objective-title').textContent='The next big batch';$('objective-text').textContent='Your bakery is open. Make dough, upgrade your oven, and build a cookie empire.';$('objective-progress').style.width='100%';}
  else if(me.coins>=250){$('objective-title').textContent='A little slice of the city';$('objective-text').textContent='You can afford a bakery plot! Find the land sign in Crumb City.';$('objective-progress').style.width='100%';}
  else if(Object.keys(inventory()).some((k)=>k!=='sugar'&&cookieNames[k]&&itemCount(k)>0)){$('objective-title').textContent='Cookies with benefits';$('objective-text').textContent='Explore the coast. Trade beach trash with Peace for 3 coins per kilogram.';$('objective-progress').style.width=Math.min(100,me.coins/250*100)+'%';}
  else{$('objective-title').textContent='Bake your own trouble';$('objective-text').textContent='Gather 2 wood, 2 sticks, and 3 stones. Press C to build your first campfire.';$('objective-progress').style.width=need/7*100+'%';}
}
function addFeed(message,kind){
  const el=document.createElement('div');el.className='feed-item '+(['hit','defeat','animal'].includes(kind)?'combat':'good');el.textContent=message;$('activity-feed').prepend(el);
  while($('activity-feed').children.length>4)$('activity-feed').lastElementChild.remove();setTimeout(()=>el.remove(),6500);
}
function toast(message,type=''){
  const el=document.createElement('div');el.className='toast '+type;el.textContent=message;$('toast-stack').append(el);
  while($('toast-stack').children.length>3)$('toast-stack').firstElementChild.remove();
  setTimeout(()=>{el.classList.add('removing');setTimeout(()=>el.remove(),300);},4400);
}
function chooseCookie(key){const slot=typeof key==='number'?key:(state.snapshot?.me.hotbar||[]).findIndex(s=>s.item===key);if(slot<0||slot>4)return;state.selectedSlot=slot;state.selected=state.snapshot?.me.hotbar?.[slot]?.item||'';action('select',{slot});updateHeldCookie();updateHUD();}
$('hotbar').addEventListener('click',(event)=>{const button=event.target.closest('[data-slot]');if(button)chooseCookie(Number(button.dataset.slot));});
function throwCookie(){
  if(!state.locked||!state.snapshot||performance.now()-state.lastThrow<160)return;
  state.lastThrow=performance.now();send({type:'input',...movement(),yaw:state.yaw,pitch:state.pitch});action('throw',{item:state.selected});state.handSwing=1;
}
function eatCookie(){if(!state.snapshot||!state.selected)return;action(['protein_powder','protein_drink'].includes(state.selected)?'consume_protein':'eat',{item:state.selected});state.handSwing=-1;}
function nearby(){
  const me=state.snapshot?.me;if(!me)return null;let found=null,best=5;
  for(const node of state.snapshot.nodes||[]){if(!node.available)continue;const d=Math.hypot(node.x-me.x,node.z-me.z);if(d<best){best=d;found={...node,distance:d,isAnimal:false};}}
  for(const animal of state.snapshot.animals||[]){if(animal.health<=0)continue;const d=Math.hypot(animal.x-me.x,animal.z-me.z);if(d<best){best=d;found={...animal,distance:d,isAnimal:true};}}
  return found;
}
function updateInteraction(){
  state.nearest=nearby();const n=state.nearest;$('interaction-prompt').hidden=!n||!state.locked;
  if(!n)return;
  const phrases={wood:['Gather wood','Campfires start with the little things'],stick:['Gather sticks','Every good batch starts somewhere'],stone:['Collect stones','For fire pits and fair trades'],dough:['Dig up cookie dough','Soft earth. Sweet discoveries.'],berry:['Pick wild berries','Try them in your next recipe'],nut:['Gather nuts','Something new for the cookbook'],cactus:['Harvest cactus','An ingredient from the sunbaked sands'],sea_salt:['Collect sea salt','A taste of the ocean'],shell:['Collect seashells','A little treasure from the tide'],trash:['Clean up the beach','Peace pays 3 coins per kilogram'],chest:['Open chest','Good things come in wooden boxes'],fire:['Bake at the campfire','Turn your finds into something delicious'],peace:['Trade with Peace','Good karma. Fair prices.'],desert_trader:['Visit Saffron','The desert has a few secret recipes'],gym:['Get a little stronger','Good cookies deserve a good throwing arm'],vending:['Use protein vending machine','Fuel your next workout or next batch'],land:['Buy bakery land','Build your little cookie empire'],bakery:['Manage your bakery','Fresh batches and brighter prospects'],dummy:['Practice your aim','Left click to throw a cookie']};
  if(n.isAnimal){$('interaction-label').textContent=n.need==='trapped'?'Free the '+pretty(n.species).toLowerCase():n.need==='wounded'?'Help the wounded '+pretty(n.species).toLowerCase():'Befriend the '+pretty(n.species).toLowerCase();$('interaction-detail').textContent=n.need==='wounded'?'Offer one '+itemName('sugar').toLowerCase()+' to help it heal':n.need==='trapped'?'A small kindness goes a long way':n.disposition==='hostile'?'This animal remembers your actions':'The ocean remembers kindness';}
  else{const p=phrases[n.kind]||[n.label,'Press E to interact'];$('interaction-label').textContent=p[0];$('interaction-detail').textContent=p[1];}
}
function interact(){
  const n=state.nearest||nearby();if(!n){toast('Move closer to something you can gather or use.');return;}
  if(n.isAnimal){action('interact',{target:n.id});return;}
  if(n.kind==='fire'){openModal('recipes',n);return;}if(n.kind==='outfit_shop'){openModal('customize',n);return;}
  if(['peace','desert_trader','gym','vending','land','bakery','bakery_plot','kitchen_shop','general_shop','paint_shop','outfit_shop'].includes(n.kind)){openModal('context',n);return;}
  action('interact',{target:n.id});
}
function capturePrompt(){if(state.player&&!state.modal&&!state.menuOpen&&state.hasPlayed)$('capture-prompt').hidden=false;}
function setPlaying(){
  if(state.sessionReplaced){location.reload();return;}
  if(!state.snapshot||!state.connected){toast('Your world is still connecting. Just a moment.');return;}
  if(!state.hasPlayed)state.zone=null;
  state.menuOpen=false;state.hasPlayed=true;$('play-overlay').hidden=true;$('start-overlay').hidden=true;$('capture-prompt').hidden=true;
  try{const promise=$('world').requestPointerLock();if(promise?.catch)promise.catch(capturePrompt);}
  catch{capturePrompt();}
}
function toggleMenu(){
  if(!state.player)return;
  if(state.menuOpen){setPlaying();return;}
  if(state.modal)closeModal({resume:false});
  state.menuOpen=true;state.keys.clear();$('start-overlay').hidden=true;$('capture-prompt').hidden=true;$('play-overlay').hidden=false;
  if(document.pointerLockElement)document.exitPointerLock();
  $('play-button').focus();
}
let escapeHandled=false;
$('play-button').addEventListener('click',setPlaying);
$('start-button').addEventListener('click',setPlaying);
$('capture-prompt').addEventListener('click',setPlaying);
$('customize-button').addEventListener('click',()=>openModal('customize'));
$('world').addEventListener('click',()=>{if(state.player&&!state.locked&&!state.modal&&!state.menuOpen)setPlaying();});
document.addEventListener('pointerlockchange',()=>{
  state.locked=document.pointerLockElement===$('world');state.playing=state.locked;
  state.keys.clear();
  if(state.locked){state.hasPlayed=true;state.menuOpen=false;$('play-overlay').hidden=true;$('start-overlay').hidden=true;$('capture-prompt').hidden=true;}
  else capturePrompt();
});
document.addEventListener('pointerlockerror',capturePrompt);
document.addEventListener('mousemove',(event)=>{if(!state.locked)return;state.yaw-=event.movementX*.002;state.pitch=clamp(state.pitch-event.movementY*.002,-1.35,1.35);});
document.addEventListener('mousedown',(event)=>{if(!state.locked)return;if(event.button===0)throwCookie();if(event.button===2)eatCookie();});
document.addEventListener('contextmenu',(event)=>{if(state.player)event.preventDefault();});
document.addEventListener('keydown',(event)=>{
  if(event.code==='Escape'){if(event.repeat)return;event.preventDefault();escapeHandled=true;toggleMenu();return;}
  if(event.target instanceof HTMLInputElement||event.target instanceof HTMLTextAreaElement||event.target instanceof HTMLSelectElement||event.ctrlKey||event.metaKey||event.altKey)return;
  if(!state.player)return;
  if(state.locked&&['KeyW','KeyA','KeyS','KeyD','ArrowUp','ArrowDown','ArrowLeft','ArrowRight','Space','ShiftLeft','ShiftRight'].includes(event.code)){event.preventDefault();state.keys.add(event.code);}
  if(event.repeat)return;
  if(event.code==='Enter'&&!state.locked&&!state.modal)setPlaying();
  if(state.menuOpen)return;
  if(event.code==='KeyI')toggleModal('inventory');
  if(event.code==='KeyC')toggleModal('recipes');
  if(event.code==='KeyM')toggleModal('map');
  if(event.code==='KeyE'&&state.locked)interact();
  if(event.code==='KeyQ'&&state.locked)eatCookie();
  if(/^Digit[1-5]$/.test(event.code)&&!state.modal)chooseCookie(Number(event.code.slice(5))-1);
});
document.addEventListener('keyup',(event)=>{state.keys.delete(event.code);if(event.code==='Escape'){if(!escapeHandled)toggleMenu();escapeHandled=false;}});
window.addEventListener('blur',()=>{state.keys.clear();escapeHandled=false;});
document.addEventListener('visibilitychange',()=>{if(document.hidden)state.keys.clear();});
$('help-button').addEventListener('click',()=>openModal('help'));
$('objective-help').addEventListener('click',()=>openModal('help'));
$('menu-button').addEventListener('click',()=>openModal('inventory'));
$('minimap-button').addEventListener('click',()=>openModal('map'));
let panels=null;
function openModal(tab,context=null){if(panels)panels.openModal(tab,context);else toast('Your field guide is loading. Try again in a moment.');}
function closeModal(options){panels?.closeModal(options);}
function toggleModal(tab){if(panels)panels.toggleModal(tab);}
function renderModal(){panels?.renderModal();}
let interactionRing;
function animate(now){
  requestAnimationFrame(animate);
  const dt=Math.min(.05,(now-(lastFrame||now))/1000);lastFrame=now;landingTime+=dt;
  const me=state.snapshot?.me;
  if(state.player&&me){
    const target=new THREE.Vector3(me.x,(me.y||0)+1.7,me.z);
    if(state.localPosition.distanceTo(target)>15)state.localPosition.copy(target);
    state.localPosition.lerp(target,Math.min(1,dt*15));
    const moving=movement();const walking=Math.hypot(moving.x,moving.z)>.05;
    const bob=walking&&state.locked?Math.sin(now*.012*(moving.sprint?1.45:1))*.027:0;
    camera.position.copy(state.localPosition);camera.position.y+=bob;camera.rotation.set(state.pitch,state.yaw,0);
    camera.fov=THREE.MathUtils.lerp(camera.fov,moving.sprint&&walking?73:68,dt*5);camera.updateProjectionMatrix();
    hand.visible=true;hand.position.set(.48,-.43+bob*.55,-.82);
    if(state.handSwing>0){hand.position.z-=Math.sin((1-state.handSwing)*Math.PI)*.36;hand.rotation.x=.55-state.handSwing*.35;state.handSwing=Math.max(0,state.handSwing-dt*4);}
    else if(state.handSwing<0){hand.position.y+=Math.sin((1+state.handSwing)*Math.PI)*.33;hand.position.x-=Math.sin((1+state.handSwing)*Math.PI)*.3;state.handSwing=Math.min(0,state.handSwing+dt*2.5);}
    else hand.rotation.x=.55;
    hand.rotation.z=-.24+(walking?Math.sin(now*.006)*.025:Math.sin(now*.002)*.009);
    if(now-lastUI>120){updateHUD();updateInteraction();lastUI=now;}
    if(now-lastPanelRefresh>1000&&(state.modal==='customize'||(state.modal==='shop'&&state.context?.kind==='outfit_shop'))){renderModal();lastPanelRefresh=now;}
    if(now-lastMinimap>150){panels?.drawMinimap();if(state.modal==='map')panels?.drawWorldMap();lastMinimap=now;}
    if(interactionRing){interactionRing.visible=!!state.nearest&&state.locked;if(state.nearest){interactionRing.position.set(state.nearest.x,.08,state.nearest.z);interactionRing.material.opacity=.36+Math.sin(now*.004)*.1;}}
  }else{
    camera.position.set(-27+Math.sin(landingTime*.05)*.55,3.4+Math.sin(landingTime*.12)*.07,-23+Math.cos(landingTime*.05)*.3);camera.lookAt(-42,2,-44);
  }
  for(const object of state.players.values()){
    object.position.lerp(object.userData.target,Math.min(1,dt*15));
    const yaw=object.userData.yaw||0;object.rotation.y=THREE.MathUtils.lerp(object.rotation.y,yaw,Math.min(1,dt*12));
  }
  for(const object of state.animals.values()){
    const target=object.userData.target;
    const dx=target.x-object.position.x,dz=target.z-object.position.z;
    if(Math.hypot(dx,dz)>.02)object.rotation.y=Math.atan2(-dx,-dz);
    object.position.lerp(target,Math.min(1,dt*8));object.position.y+=Math.sin(now*.0017+object.position.x)*.002;
    const animal=object.userData.animal;if(Number.isFinite(animal.heading))object.rotation.y=animal.heading;
    const plastic=object.getObjectByName('rescue-plastic');if(plastic)plastic.visible=animal.need==='trapped';const wound=object.getObjectByName('animal-wound');if(wound)wound.visible=animal.need==='wounded'; const animalStatus=animal.need+'|'+animal.disposition; if(object.userData.lastNeed!==animalStatus){
      object.userData.lastNeed=animalStatus;const old=object.userData.label;if(old){object.remove(old);old.material.map.dispose();old.material.dispose();}
      const label=textSprite(pretty(animal.species)+(animal.need?' · '+pretty(animal.need):animal.disposition==='hostile'?' · ANGRY':''),'#fff3dc','#294b43b0',512,90);label.position.y=(animal.species==='whale'?3:1.4);label.scale.set(2.8,.5,1);object.add(label);object.userData.label=label;
    }
    if(object.userData.label)object.userData.label.visible=!!me&&Math.hypot(animal.x-me.x,animal.z-me.z)<22;
  }
  const prediction=Math.min(.15,(now-state.lastSnapshot)/1000);
  for(const object of state.projectiles.values()){
    object.position.copy(object.userData.target).addScaledVector(object.userData.velocity,prediction);object.rotation.x+=dt*10;object.rotation.z+=dt*6;
  }
  for(const object of state.nodes.values()){
    if(object.userData.node.kind==='fire'){const flame=object.getObjectByName('flame'),f2=object.getObjectByName('flame2');if(flame){flame.scale.y=.8+Math.sin(now*.014+object.position.x)*.2;flame.rotation.y=now*.002;}if(f2)f2.scale.y=.85+Math.cos(now*.018)*.15;}
  }
  if(water&&now-lastAnimation>60){const positions=water.geometry.attributes.position.array;for(let i=0;i<positions.length;i+=3)positions[i+1]=Math.sin(waterPositions[i]*.22+now*.00085)*.052+Math.cos(waterPositions[i+2]*.21+now*.0006)*.035;water.geometry.attributes.position.needsUpdate=true;lastAnimation=now;}
  renderer.render(scene,camera);
}
window.addEventListener('resize',()=>{if(!renderer)return;camera.aspect=innerWidth/innerHeight;camera.updateProjectionMatrix();renderer.setSize(innerWidth,innerHeight);});
$('world').addEventListener('webglcontextlost',(event)=>{event.preventDefault();toast('The graphics connection was interrupted. Reload to return to your world.','error');});
Object.assign(buffNames,{slowed:'SLOWED',weakened:'WEAKER THROWS',spawn_shield:'FRESHLY BAKED · PROTECTED'});
initializeRenderer();
if(scene){interactionRing=new THREE.Mesh(new THREE.TorusGeometry(.8,.022,5,40),new THREE.MeshBasicMaterial({color:'#f6d299',transparent:true,opacity:.45,depthWrite:false}));interactionRing.rotation.x=-Math.PI/2;interactionRing.visible=false;scene.add(interactionRing);}
import('./panels.js').then(({createPanels})=>{panels=createPanels({$,state,action,toast,chooseCookie,itemCount,itemName,cookieName,cookieNote,isKnown,cookieIcon,itemIcon,materialEmoji,cookieNames,cookieNotes,escapeHTML,pretty,resume:setPlaying});}).catch((error)=>{console.warn('Field guide unavailable',error);toast('The field guide could not load. Please reload the page.','error');});
api('/api/world').then(({layout})=>{if(layout&&scene&&state.layout?.version!==layout.version)createEnvironment(layout);}).catch(()=>{});
api('/api/session').then((session)=>{if(session.authenticated&&session.player)enterGame(session.player);}).catch(()=>{$('server-status').textContent='CONNECTING TO THE WORLD';});
if(matchMedia('(pointer: coarse)').matches){$('mobile-note').hidden=false;}
