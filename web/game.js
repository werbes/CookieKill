import * as THREE from './vendor/three.module.js';
import {createSnapshotDecoder} from './state-sync.js';
import {createIslandScene} from './island-scene.js';
import {buildCave,caveHeight,cutCaveOpening} from './cave-scenery.js';
import {buildForestClub,cutClubOpening} from './club-scenery.js';
import {createBakeryScene} from './bakery-scene.js';
import {bakingItemName,bakingIngredientIcons,isDoughItem,createDoughView} from './baking-items.js';
import {buildFlatRoof} from './roof-scenery.js';
import {createProductionScene} from './production-scenery.js';
import {createNightScene,nightFactor} from './night-scene.js';
import {createFunnyCakeEffects} from './funny-cake-effects.js';
import {buildJohnHut} from './john-hut-scenery.js';
import {buildBeachRaceCave,createCamelRaceScene} from './camel-race-scene.js';
import {createCamelRaceHUD} from './camel-race-panels.js';
import {buildCamelTrainingEntrance,createCamelTrainingScene} from './camel-training-scene.js';
import {createCamelTrainingHUD} from './camel-training-panels.js';
import {buildAchievementHat,createAchievementScenery} from './achievement-scenery.js';
import {createBouquetView} from './bouquet-view.js';
import {createCamelFoodView,animateCamelView,heldCamelTreat,heldTreatTarget} from './camel-view.js';
import {createTownScenery} from './town-scenery.js';
import {createTreeRingScene} from './tree-ring-scene.js';
import {createCookiePracticeScene} from './cookie-practice-scene.js';
import {createPracticeAvatarRig,animatePracticeAvatar} from './practice-avatar.js';
import {createCookieActivityHUD} from './cookie-activities-panels.js';
import {createAmbientAudio} from './ambient-audio.js';
import {buildBakeryRoom} from './bakery-room.js';
import {createBakeryFurnishings,insideOwnBakery,ownedBakery,bakeryPlacementValid} from './bakery-furnishings.js';
import {clubReducedFlashing,setClubReducedFlashing} from './club-settings.js';
import {createFlowerScenery} from './flower-scenery.js';
import {createTraderScenery} from './trader-scenery.js';
import {createSceneryBatches} from './scenery-batches.js';
import {createMotionClock,createMotionTrack,LOCAL_MOTION_DELAY_MS,REMOTE_MOTION_DELAY_MS} from './motion.js';

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
const materialEmoji = {flower_daisy:'🌼',flower_tulip:'🌷',flower_lavender:'✿',bouquet_daisies:'🌼',bouquet_tulips:'🌷',bouquet_lavender:'✿',flower_bouquet:'💐',swift_dates:'●',spring_carrot:'🥕',stamina_oats:'🌾',happy_apple:'🍎',funny_cake:'🍰',cosmic_cake:'🍰',wood:'🪵',stick:'╱',stone:'◈',dough:'◕',berry:'●',nut:'◉',cactus:'♧',sea_salt:'✧',shell:'🐚',trash:'♻',pearl:'○',treasure:'♜',old_coin:'◎',protein_powder:'▥',protein_drink:'▤'};
const buffNames = {speed:'QUICK FEET',slow:'SLOWED',weaken:'WEAKER THROWS',weak:'WEAKER THROWS',range:'LONG THROW',short:'SHORT THROW',swim:'OCEAN LEGS',superstrength:'SUPERSTRENGTH',protein:'PROTEIN BOOST',training:'TRAINING BOOST',funny_cake:'FUNNY CAKE'};
const state = {player:null,snapshot:null,ws:null,connected:false,playing:false,locked:false,modal:null,menuOpen:false,context:null,email:'',yaw:0,pitch:0,keys:new Set(),lastInput:0,selected:'sugar',selectedSlot:0,hotkeys:[],nodes:new Map(),animals:new Map(),players:new Map(),homes:new Map(),projectiles:new Map(),events:new Set(),nearest:null,lastHealth:100,localPosition:new THREE.Vector3(-35,1.7,-35),handSwing:0,lastThrow:0,lastSnapshot:0,hasPlayed:false,wasModalLocked:false,worldReady:false,serverRecipes:[],intentYaw:false};
let renderer, scene, camera, clock, worldGroup, hand, water, waterPositions, landingTime=0, lastUI=0, lastMinimap=0, lastFrame=0, lastAnimation=0, lastPanelRefresh=0, miniContext;
let islandScene, buildGhost, riddenCamel, flowerScenery, caveScenery, forestClub, bakeryScene, bakeryFurnishings, productionScene, nightScene, funnyCakeEffects, daylight, skylight;
const nearForestClub=me=>!!me&&!me.island&&me.x>-229&&me.x<-175&&me.z<-184;
const cakeEffectActive=me=>(me?.buffs?.funny_cake||0)>0;
let raceScene,raceHUD,townScenery,trainingScene,trainingHUD,achievementScenery,treeRingScene,practiceScene,cookieActivityHUD;
const camelArena=island=>island==='camel_race'||island==='camel_training';
const cookieArena=island=>island==='cookie_ring'||island==='cookie_practice';
const activityArena=island=>camelArena(island)||cookieArena(island);
const ambientAudio=createAmbientAudio();
const motionClock=createMotionClock(),localMotion=createMotionTrack(),localPose={};
const motionIdentity=p=>`${p.island||''}|${p.deaths||0}|${p.posture||''}|${p.ridingCamel?1:0}`;
const eyeHeight=p=>p.posture==='lying'?.5:p.posture==='sitting'?1.05:p.ridingCamel?3.1:1.7;
const dynamicGroup = new THREE.Group();
const buildGhostTints={valid:new THREE.Color('#b3e8b7'),invalid:new THREE.Color('#ed9b8b')};
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
let traderScenery;
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
function addTree(x,z,scale=1,kind=0,color=null,variant=0) {
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
  if(color)group.traverse(o=>{if(o.isMesh&&o.geometry.type==='IcosahedronGeometry')o.material=mat(color);});
  group.rotation.y=variant*.83;group.scale.x*=1+(variant%3)*.08;
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
  if(type==='bakery'){buildBakeryRoom({THREE,parent:g,box,sphere,cylinder,mat,flatLabel},options);return g;}
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
  buildFlatRoof({box,parent:g},{width:w,depth:d,wallTop:h,color:desert?'#b58a5f':'#79735c',name:'building-roof'});
  for(const child of g.children)if(child.material===mat(wall))child.userData.paintTarget='walls';
  const rug=box(door+1,.025,d-1,desert?'#aa6650':bakery?'#ac8060':'#82917c',0,.09,0,g);
  for(const side of [-1,1]) {
    box(.35,.7,.26,'#936c3f',side*(w/2-.65),2.4,-d/2+.3,g);
    const lantern=sphere(.15,'#ffe2a0',side*(w/2-.65),2.5,-d/2+.52,g);
    lantern.material=mat('#ffe2a0',{emissive:'#efb559',emissiveIntensity:.6});
  }
  if(desert) {
    if(options.label){const sign=flatLabel(options.label.replace('BARTER','INGREDIENTS'),'#674d36','#efdbaf',768,90);sign.position.set(0,h-.45,d/2+.17);sign.scale.set(w-.6,.62,1);g.add(sign);}
    const awning=box(4,.12,2.4,'#c67e4c',0,3,d/2+1.2,g);awning.rotation.x=.08;
    cylinder(.06,.07,3,'#8d7146',-1.9,1.5,d/2+2.2,g);cylinder(.06,.07,3,'#8d7146',1.9,1.5,d/2+2.2,g);
    sphere(.5,'#b97650',-3,.5,d/2+.5,g);cylinder(.4,.5,.7,'#b7774e',-3,.8,d/2+.5,g);
    for(const side of [-1,1]) box(1,1.15,.12,'#7a8066',side*2.6,2.4,d/2+.05,g);
  }else{
    for(const side of [-1,1]){box(2.1,1.8,.14,'#6f9990',side*(w/2-1.8),2.5,d/2+.08,g);box(.09,1.8,.2,'#eddfbb',side*(w/2-1.8),2.5,d/2+.16,g);}
    if(bakery){const a=box(7,.12,2.6,'#df996a',0,3.7,d/2+1.2,g);a.rotation.x=.12;for(let i=0;i<7;i++)box(.5,.12,2.65,'#f6e6bd',-3+i,3.73,d/2+1.2,g);}
    if(bakery||gym||shop){const sign=flatLabel(options.label||(bakery?'BAKERY PLOT':gym?'CRUMB CITY GYM':type==='kitchen_shop'?'KETTLE & COLOUR':'THE CORNER PANTRY'),bakery?'#74482d':'#354f43','#f3e6c5',512,90);sign.position.set(0,h-.65,d/2+.17);sign.scale.set(Math.min(w-1,9),1.1,1);g.add(sign);g.userData.sign=sign;}
    if(gym){for(const side of [-1,1]){box(1.2,.25,2.7,'#546a60',side*3,.5,-2,g);box(.09,1,.1,'#949989',side*3-.5,.9,-3,g);box(.09,1,.1,'#949989',side*3+.5,.9,-3,g);}box(4,.2,.85,'#bd9b70',0,.55,-2.8,g);}
    if(gym){
      for(const side of [-1,1]){box(2.2,2.35,.04,'#486c72',side*(w/2-1.6),1.75,d/2+.14,g);box(1.25,.075,.12,'#ddd9c0',side*(w/2-1.6),1.65,d/2+.21,g);for(const end of [-1,1])cylinder(.3,.3,.14,'#253c40',side*(w/2-1.6)+end*.51,1.65,d/2+.25,g,12).rotation.z=Math.PI/2;}
    }
    if(shop){box(w+.6,.22,1.8,type==='kitchen_shop'?'#7d9c88':'#d6ad67',0,3.45,d/2+.75,g);for(const side of [-1,1]){box(2.1,1.8,.05,'#658c8d',side*(w/2-1.6),2,d/2+.17,g);box(2,.15,.8,'#926a4c',side*(w/2-1.6),1,d/2+.5,g);}if(type==='kitchen_shop')for(let i=0;i<3;i++)cylinder(.22,.22,.35,['#c86656','#85a085','#d9b765'][i],-w/2+1+i*.5,1.28,d/2+.5,g);}
  }
  if(type==='ingredient_shop'){
    for(const side of [-1,1]){box(3.4,.12,.8,'#98734f',side*3.5,1,-2.5,g);box(3.4,.12,.8,'#98734f',side*3.5,2,-2.5,g);for(let i=0;i<5;i++){box(.38,.6,.3,side===-1?'#ece0c2':'#dbc27c',side*3.5-1.2+i*.6,1.36,-2.5,g);box(.38,.55,.3,side===-1?'#b5c5a2':'#e2c3a2',side*3.5-1.2+i*.6,2.34,-2.5,g);}}
    const label=flatLabel('SUGAR  BUTTER  FLOUR  EGGS','#5b684c','#ece4c9',768,90);label.position.set(0,3.4,-3.8);label.scale.set(7,.7,1);g.add(label);
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
let sceneryBatches;
function createEnvironment(layout=null) {
  treeRingScene?.dispose();treeRingScene=null;townScenery?.dispose();townScenery=null;achievementScenery?.dispose();achievementScenery=null;
  if(layout)state.layout=layout;
  sceneryBatches?.dispose();sceneryBatches=null;
  for(const object of [...worldGroup.children]){worldGroup.remove(object);disposeObject(object);}scenery.clear();
  const batchRoots=[];
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
  cutCaveOpening(geometry,THREE);
  cutClubOpening(geometry,THREE);
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
    if(p.kind==='tree'||p.kind==='pine')o=addTree(p.x,p.z,s,p.kind==='pine'?1:0,p.color,p.variant);
    else if(p.kind==='palm'){addPalm(p.x,p.z,s);o=worldGroup.children.at(-1);}
    else if(p.kind==='rock'||p.kind==='beach_rock'){o=addRock(p.x,p.z,s);if(p.kind==='beach_rock')o.material=mat('#cbb68e');}
    else if(p.kind==='bush'){o=sphere(s,'#6f8950',p.x,s*.48,p.z);o.scale.set(1,.7,.9);}
    else if(p.kind==='cactus'){o=new THREE.Group();worldGroup.add(o);o.position.set(p.x,0,p.z);buildCactus(o,s,p.color,p.variant);}
    else if(p.kind.startsWith('cave_')||p.kind.startsWith('club_')||p.kind.startsWith('town_')||p.kind==='john_pork_hut'||p.kind==='camel_race_cave'||p.kind==='tree_hut')continue;
    else if(p.kind==='city_plant'||p.kind==='plant'){o=new THREE.Group();worldGroup.add(o);o.position.set(p.x,0,p.z);cylinder(.45,.32,.6,'#b58162',0,.3,0,o);for(let i=0;i<5;i++)flowerScenery.flower(o,Math.sin(i*2.4)*.25,.6,Math.cos(i*2.4)*.25,i+(p.variant||0),.85);}
    else o=addBuilding(p.x,p.z,p.kind,p);
    if(o){o.userData.prop=p;scenery.set(p.id,o);if(['tree','pine','palm','rock','beach_rock','bush','cactus','city_plant','plant'].includes(p.kind))batchRoots.push(o);}
  }
  for(const zone of state.layout?.zones||[]){
    const sign=new THREE.Group();worldGroup.add(sign);sign.position.set(zone.x-zone.width/2+2,0,zone.z+zone.depth/2+1);
    cylinder(.075,.09,1.6,'#8a6a47',0,.8,0,sign);box(2.6,.7,.12,'#ab8955',0,1.4,0,sign);
    const label=flatLabel(zone.name,'#fff0d0');label.position.set(0,1.43,.08);label.scale.set(2.5,.48,1);sign.add(label);
  }
  const decorationStart=worldGroup.children.length,flowers=rng(2746);
  for(let i=0;i<520;i++){const x=-235+flowers()*228,z=-253+flowers()*278;if(z>coastAt(x)-22)continue;const blocked=(x>-211&&x<-193&&z<-195&&z>-227)||(state.layout?.zones||[]).some(p=>Math.abs(x-p.x)<p.width/2&&Math.abs(z-p.z)<p.depth/2);if(!blocked){if(i%6===0)addMushroom(x,z,.6+flowers());else addGrass(x,z,'#718e4b',.5+flowers());if(i%3===0)flowerScenery.patch(worldGroup,x,z,i,.75+flowers()*.4);}}
  for(let i=0;i<44;i++){const a=i*2.4,r=5+(i*3.73)%24;flowerScenery.patch(worldGroup,-35+Math.cos(a)*r,-35+Math.sin(a)*r,i,.85+(i%3)*.13);}
  for(const [i,p] of (state.layout?.plots||[]).entries())for(const sign of [-1,1])flowerScenery.bed(worldGroup,p.x+sign*(p.width/2-1.45),p.z+p.depth/2+.8,i+(sign===1?2:0));
  for(let i=0;i<22;i++)addCloud(-230+random()*470,40+random()*30,-240+random()*450,1+random());
  batchRoots.push(...worldGroup.children.slice(decorationStart));
  sceneryBatches=createSceneryBatches(THREE,worldGroup,batchRoots);
  const sun=sphere(6,'#fff0c4',-80,70,-245);sun.material=new THREE.MeshBasicMaterial({color:'#ffeebd'});sun.castShadow=false;
  createCave();
  forestClub=buildForestClub({THREE,parent:worldGroup,layout:state.layout,box,sphere,cylinder,mesh,mat,flatLabel,makePerson});
  buildJohnHut({THREE,parent:worldGroup,flatLabel});
  buildBeachRaceCave({THREE,parent:worldGroup,flatLabel});
  buildCamelTrainingEntrance({THREE,parent:worldGroup,flatLabel});
  treeRingScene=createTreeRingScene({THREE,scene,worldParent:worldGroup,flatLabel});
  achievementScenery=createAchievementScenery({THREE,parent:worldGroup,box,cylinder,flatLabel,disposeObject});
  townScenery=createTownScenery({THREE,parent:worldGroup,flatLabel});
  worldGroup.visible=!state.snapshot?.me.island;
}
function buildCactus(g,scale=1,color=null,variant=0){
  cylinder(.25,.3,2.3,'#7b9b61',0,1.15,0,g,6);sphere(.25,'#86a86d',0,2.28,0,g);
  const arm=cylinder(.14,.18,.8,'#77975b',.43,1.1,0,g,6);arm.rotation.z=Math.PI/2;
  cylinder(.15,.18,.9,'#89a36c',.77,1.5,0,g,6);sphere(.16,'#91af74',.77,1.95,0,g);g.scale.setScalar(scale);
  if(variant%2){const arm2=cylinder(.14,.17,.8,'#719166',-.48,.9,0,g);arm2.rotation.z=Math.PI/2;cylinder(.16,.18,1.3,'#779768',-.85,1.4,0,g);sphere(.17,'#86a472',-.85,2.05,0,g);}
  if(color)g.traverse(o=>{if(o.isMesh)o.material=mat(color);});g.rotation.y=variant*.9;g.scale.y*=1+(variant%3)*.13;
  flowerScenery.cactus(g,variant);
}
function createCave(){caveScenery=buildCave({THREE,parent:worldGroup,layout:state.layout,box,sphere,cylinder,mesh,mat,flatLabel});createSceneryBatches(THREE,caveScenery,[...caveScenery.children].filter(o=>!o.isInstancedMesh));return caveScenery;}
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
function makePerson(name='',color='#df9d60',peace=false,avatar={},heldItem='') {
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
  const bouquet=createBouquetView({THREE,sphere,box,cylinder},heldItem),held=bouquet||makeCookie('sugar',.45);held.position.set(.37,.97,-.2);if(bouquet)held.rotation.z=-.15;g.add(held);
  if(peace){cylinder(.5,.5,.06,'#dcb782',0,1.85,0,g,10);cylinder(.23,.3,.24,'#d5ad78',0,1.95,0,g,8);box(.4,.12,.08,'#cf8b70',0,1.1,-.23,g);}
  else if(avatar.hat&&avatar.hat!=='none')buildAchievementHat({THREE,parent:g,box,sphere,cylinder},avatar.hat);
  if(name){const label=textSprite(name,'#fff6db','#233f2cca',512,96);label.position.y=avatar.hat?2.85:2.3;label.scale.set(2.2,.42,1);g.add(label);}
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
    buildFlatRoof({box,parent:g},{width:w,depth:d,wallTop:3.6,color:'#657d63',name:'cabin-roof'});
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
  const paint=n.paint||{},level=n.bakeryLevel||0,signature=JSON.stringify([n.owner,n.label,n.username,paint,level]);if(building.userData.bakerySignature===signature)return;building.userData.bakerySignature=signature;
  if(building.userData.sign){building.remove(building.userData.sign);disposeObject(building.userData.sign);}
  const sign=flatLabel(n.owner?(n.label||'BAKERY'):'AVAILABLE BAKERY','#74482d','#f3e6c5',768,100);sign.position.set(0,5.15,8.19);sign.scale.set(9,.85,1);building.add(sign);building.userData.sign=sign;
  const old=building.getObjectByName('bakery-custom');if(old){building.remove(old);disposeObject(old);}
  const custom=new THREE.Group();custom.name='bakery-custom';building.add(custom);
  if(n.owner){const username=flatLabel('@'+(n.username||'baker')+(level?' - OVEN LEVEL '+level:''),'#605c40','#eaddb7',512,75);username.position.set(0,3.85,8.21);username.scale.set(5,.45,1);custom.add(username);}
  building.traverse(o=>{if(o.userData.paintTarget==='walls')o.material=mat(avatarColor(paint.walls,'#e6d6b0'));});
}
function updateCamelName(object,node){
  if(!object||object.userData.camelName===node.label)return;
  for(const child of [...object.children])if(child.isSprite){object.remove(child);disposeObject(child);}
  const label=textSprite(node.label||'Camel','#f5e2bd','#654d3d99');label.position.y=3.25;label.scale.set(2.4,.4,1);object.add(label);object.userData.camelName=node.label;
}
function nodeObject(n) {
  const g=new THREE.Group();
  switch(n.kind){
    case 'ingredient_shop':{const vendor=makePerson('Baker\u2019s Pantry','#bd9562',false,{shirt:'tee',shirtColor:'sand',pantsColor:'black',hat:'chef'});g.add(vendor);break;}
    case 'tree_golden_cookie':case 'tree_silver_cookie':case 'cookie_ring_exit':case 'cookie_practice_exit':case 'practice_treadmill':case 'practice_bench_press':case 'practice_leg_press':case 'practice_pool':case 'practice_target':case 'bakery_chair':case 'club_bar':case 'club_casino':case 'club_seat':case 'club_cake_seller':case 'camel_training_lollipop':case 'camel_race_lollipop':case 'flower_shop':case 'pet_shop':case 'betty':case 'leaderboard_board':case 'flower_daisy':case 'flower_tulip':case 'flower_lavender':break; // Dedicated scenery owns these models.
    case 'item_drop': {
      const count=n.amount||1,size=count>=10?.72:count>=5?.5:.24;
      if(count===1&&cookieNames[n.item]){const cookie=makeCookie(n.item,.7);cookie.position.y=.16;g.add(cookie);}
      else if(count===1&&n.item==='wood'){const log=cylinder(.12,.14,.65,'#997346',0,.15,0,g,7);log.rotation.z=Math.PI/2;}
      else if(count===1&&n.item==='stone')addRock(0,0,.24,g);
      else{box(size,size*.7,size*.7,count>=10?'#aa794c':count>=5?'#c4a16d':'#d6b78c',0,size*.35,0,g);if(count>1){box(size+.025,.055,size*.72,'#f2d197',0,size*.38,0,g);box(.065,size*.72,size*.73,'#f2d197',0,size*.36,0,g);}}
      if(count>1){const label=textSprite(String(count),'#fff3cc','#493e32cc');label.position.y=size+.18;label.scale.set(.4,.24,1);g.add(label);}break;
    }
    case 'wood': {if(!scenery.has(n.id)){const tree=addTree(0,0,n.scale||1,n.variant%3===1?1:0,n.color,n.variant);worldGroup.remove(tree);g.add(tree);}break;}
    case 'stick':for(let i=0;i<4;i++){const m=cylinder(.035,.06,1.25,'#927a48',(i-2)*.12,.12,0,g,5);m.rotation.set(.2*i,.3*i,Math.PI/2-.1*i);}break;
    case 'stone':{const a=addRock(-.25,0,.35,g),b=addRock(.3,.1,.28,g);if(n.x<0&&n.z>coastAt(n.x)-18)a.material=b.material=mat('#cab590');break;}
    case 'berry':sphere(.65,'#647f48',0,.5,0,g,1);sphere(.5,'#79924e',.4,.45,.1,g);for(let i=0;i<10;i++){const a=i*2.4;sphere(.105,'#b7504f',Math.cos(a)*.53,.65+(i%3)*.13,Math.sin(a)*.5,g);}break;
    case 'nut':sphere(.55,'#829355',0,.42,0,g);for(let i=0;i<7;i++)sphere(.15,'#9c7047',Math.cos(i)*.5,.13,Math.sin(i)*.5,g);break;
    case 'cactus':if(!scenery.has(n.id))buildCactus(g,n.scale||1,n.color,n.variant);break;
    case 'pink_lollipop': case 'blue_lollipop': {
      const color=n.kind==='pink_lollipop'?'#f4a8c5':'#88c9ee';
      cylinder(.035,.035,.7,'#f5ead5',0,.35,0,g);const sweet=sphere(.22,color,0,.83,0,g,2);sweet.scale.z=.35;
      mesh(new THREE.TorusGeometry(.12,.025,6,20),'#fff1dc',0,.83,.075,g);
      const label=textSprite(n.kind==='pink_lollipop'?'A taste of home':'A taste of friendship',color,'#463e4588');label.position.y=1.4;label.scale.set(2.5,.49,1);g.add(label);break;
    }
    case 'abu_fanous': {g.add(makePerson('ABU FANOUS','#d9bf87'));const camel=islandScene?.camel();if(camel){camel.position.set(3,0,-1);camel.rotation.y=-.7;g.add(camel);}box(4,.12,3,'#bc8b65',0,3,-2,g);for(const x of [-1.8,1.8])cylinder(.08,.1,3,'#8b6e4d',x,1.5,-2,g);break;}
    case 'camel': {g.add(islandScene.camel());const label=textSprite(n.label||'Camel','#f5e2bd','#654d3d99');label.position.y=3.25;label.scale.set(2.4,.4,1);g.add(label);break;}
    case 'island_chest': {box(1.6,.8,1,'#a07a51',0,.4,0,g);box(1.65,.17,1.06,'#bd935f',0,.85,0,g);box(.2,.25,.06,'#e6c77b',0,.65,.55,g);break;}
    case 'bakery_fridge': case 'island_upgrade': case 'island_portal': case 'bakery_oven': case 'bakery_mixer': case 'bakery_cooling_table': case 'bakery_display': break;
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
    case 'bakery_plot':{const menu=flatLabel('CLAIM / MANAGE BAKERY','#714e33','#edddaf',384,80);menu.position.set(0,1.7,0);menu.scale.set(2.1,.5,1);g.add(menu);cylinder(.055,.07,1.5,'#8d724e',0,.75,0,g);g.position.set(n.x-2.7,0,n.z+8.6);break;}
    case 'gym':{const bar=cylinder(.07,.07,2,'#6d706b',0,.65,0,g,8);bar.rotation.z=Math.PI/2;for(const side of [-1,1]){const weight=cylinder(.4,.4,.25,'#536b63',side*.75,.65,0,g,10);weight.rotation.z=Math.PI/2;}break;}
    case 'vending':box(1.3,2.2,.8,'#81928a',0,1.1,0,g);box(.93,1.23,.1,'#4c7067',-.07,1.36,.46,g);for(let i=0;i<6;i++)cylinder(.075,.07,.24,['#e3c398','#b493bc','#d4d5a5'][i%3],-.33+(i%3)*.29,1.16+Math.floor(i/3)*.46,.55,g);box(.7,.17,.12,'#294d43',0,.37,.45,g);break;
    case 'land':cylinder(.05,.07,1.7,'#897343',0,.85,0,g,5);box(1.65,.8,.12,'#ebd8a8',0,1.45,0,g);{const sign=flatLabel('YOUR BAKERY HERE','#6c7850',null,512,120);sign.position.set(0,1.47,.08);sign.scale.set(1.55,.37,1);g.add(sign);}break;
    case 'bakery':box(2,.8,1,'#bb9162',0,.4,0,g);for(let i=0;i<3;i++){const c=makeCookie(i===2?'cake':'sugar',1);c.position.set((i-1)*.55,.9,0);g.add(c);}break;
    case 'dummy':cylinder(.09,.14,1.5,'#937546',0,.75,0,g,6);{const target=cylinder(.5,.5,.2,'#d5a65e',0,1.6,0,g,12);target.rotation.x=Math.PI/2;const ring=mesh(new THREE.TorusGeometry(.31,.025,4,16),'#a16343',0,1.6,.12,g);sphere(.075,'#a16343',0,1.6,.12,g);}break;
    default:sphere(.35,'#ccb477',0,.3,0,g);
  }
  if(n.kind==='abu_fanous')traderScenery?.decorateAbu(g);
  if(n.kind==='peace')traderScenery?.decoratePeace(g);
  g.position.set(n.x,n.y??(['pink_lollipop','blue_lollipop'].includes(n.kind)?caveHeight(n.x,n.z):0),n.z);if(n.kind==='bakery_plot')g.position.set(n.x-2.7,0,n.z+8.6);g.userData.node=n;
  return g;
}
function makeAnimal(a){
  const g=new THREE.Group(),s=a.species;
  if(a.egg){const egg=sphere(.3,'#e7dfb9',0,.25,0,g,2);egg.scale.set(.8,1.3,.8);for(let i=0;i<5;i++)sphere(.035,'#baad81',Math.sin(i*2.4)*.2,.25+Math.cos(i)*.25,.15,g);const label=textSprite('Egg · leave undisturbed','#fff3dc','#294b43b0');label.position.y=1.3;label.scale.set(2.8,.5,1);g.add(label);g.userData.label=label;return g;}
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
    skylight=new THREE.HemisphereLight('#f9efd3','#718b5b',2.15);scene.add(skylight);
    const sun=new THREE.DirectionalLight('#ffe4ac',3.25);sun.position.set(-50,70,-30);sun.castShadow=true;sun.shadow.mapSize.set(2048,2048);sun.shadow.camera.left=-65;sun.shadow.camera.right=65;sun.shadow.camera.top=65;sun.shadow.camera.bottom=-65;sun.shadow.camera.far=160;sun.shadow.bias=-.0004;sun.shadow.normalBias=.055;scene.add(sun);sun.target.position.set(-22,0,-20);scene.add(sun.target);
    daylight=sun;flowerScenery=createFlowerScenery({THREE,mesh,sphere,cylinder,box,mat});worldGroup=new THREE.Group();scene.add(worldGroup);scene.add(dynamicGroup);createEnvironment();
    islandScene=createIslandScene({THREE,scene,box,sphere,cylinder,mesh,mat,textSprite,disposeObject,makeAnimal});
    riddenCamel=islandScene.camel();riddenCamel.visible=false;scene.add(riddenCamel);
    traderScenery=createTraderScenery({THREE,box,sphere,cylinder,mesh,mat,flatLabel,textSprite});
    bakeryFurnishings=createBakeryFurnishings({THREE,box,cylinder,sphere,flatLabel,disposeObject});
    bakeryScene=createBakeryScene({THREE,parent:scene,makePerson,makeCookie,makeDough:(item,scale)=>createDoughView({THREE,sphere,box},item,scale),box,cylinder,flatLabel,disposeObject});
    productionScene=createProductionScene({THREE,parent:scene,makeCookie,makeDough:(item,scale)=>createDoughView({THREE,sphere,box},item,scale),flatLabel,disposeObject});
    nightScene=createNightScene({THREE,parent:scene,box,sphere,cylinder,disposeObject});
    funnyCakeEffects=createFunnyCakeEffects({canvas:renderer.domElement});
    raceScene=createCamelRaceScene({THREE,scene,flatLabel});
    raceHUD=createCamelRaceHUD({document,state,openModal});
    trainingScene=createCamelTrainingScene({THREE,scene,flatLabel});trainingHUD=createCamelTrainingHUD({document,state,openModal});
    practiceScene=createCookiePracticeScene({THREE,scene,flatLabel});cookieActivityHUD=createCookieActivityHUD({document,state,openModal});
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
  try{const result=await api('/api/verify',{email:state.email,code:$('login-code').value.trim(),keepSignedIn:$('keep-signed-in').checked});enterGame(result.player);}
  catch(error){authMessage(error.message);}finally{button.disabled=false;}
});
$('change-email').addEventListener('click',()=>{$('email-stage').hidden=false;$('code-stage').hidden=true;authMessage('');$('email').focus();});
$('logout-button').addEventListener('click',async()=>{
  try{await api('/api/logout',{});nightScene?.clear();ambientAudio.clear();state.player=null;state.connected=false;if(state.ws)state.ws.close();location.reload();}catch(error){toast(error.message,'error');}
});
function enterGame(player){
  state.player=player;$('landing').hidden=true;document.body.classList.add('playing');$('hud').hidden=false;$('player-name').textContent=player.callName||player.name;$('start-overlay').hidden=false;
  connect();
}
let reconnectTimer;
function connect(){
  if(!state.player||state.sessionReplaced)return;
  clearTimeout(reconnectTimer);
  const decoder=createSnapshotDecoder();
  const ws=new WebSocket((location.protocol==='https:'?'wss://':'ws://')+location.host+'/ws?updates=delta-v1');state.ws=ws;
  ws.addEventListener('open',()=>{if(state.ws!==ws)return;state.connected=true;$('disconnect-banner').hidden=true;$('connection-state').innerHTML='<i></i> CONNECTED';});
  ws.addEventListener('message',(event)=>{
    if(state.ws!==ws||ws.readyState!==WebSocket.OPEN)return;
    let snapshot;
    try{
      const message=JSON.parse(event.data);
      if(message.type==='error'){state.pendingBuildEdit=null;state.pendingBakeryEdit=null;state.buildOperationPending=false;panels?.actionFailed?.(message.message);toast(message.message||'That action is not available.','error');return;}
      snapshot=decoder.apply(message);
    }catch(error){console.warn('Could not read game update; reconnecting',error);state.connected=false;ws.close(4000,'Game state resync');return;}
    if(snapshot)receiveSnapshot(snapshot);
  });
  ws.addEventListener('close',(event)=>{
    if(state.ws!==ws||!state.player)return;
    panels?.connectionLost?.();
    nightScene?.clear();ambientAudio.clear();
    state.connected=false;state.keys.clear();$('disconnect-banner').hidden=false;$('connection-state').textContent='RECONNECTING';
    if(event.code===4001||event.code===1008){
      state.sessionReplaced=true;if(document.pointerLockElement)document.exitPointerLock();
      const message=event.code===4001?'Account opened in another tab. Reload to reconnect.':'Your session ended. Reload to sign in again.';
      $('disconnect-banner').textContent=message;$('connection-state').textContent='DISCONNECTED';toast(message,'error');return;
    }
    reconnectTimer=setTimeout(connect,2000);
  });
  ws.addEventListener('error',()=>{if(state.ws===ws)state.connected=false;});
}
function send(message){if(state.ws&&state.ws.readyState===WebSocket.OPEN){state.ws.send(JSON.stringify(message));return true;}return false;}
function action(actionName,extra={}){
  if(!state.connected){toast(state.sessionReplaced?'Account opened in another tab. Reload to reconnect.':'Reconnect to the world to do that.','error');return;}
  if(state.snapshot?.me.fallen&&actionName!=='stand'){toast(fallPrompt(state.snapshot.me));return;}
  send({type:'action',action:actionName,...extra});
}
function fallPrompt(me){return me.standIn>0?'You fell over. Rest for '+me.standIn.toFixed(1)+'s before standing.':'Press E, Space, or move to stand up.';}
function movement(){
  if(!state.locked||state.modal||!state.connected)return{x:0,z:0,sprint:false,jump:false};
  const forward=(state.keys.has('KeyW')||state.keys.has('ArrowUp')?1:0)-(state.keys.has('KeyS')||state.keys.has('ArrowDown')?1:0);
  const side=(state.keys.has('KeyD')||state.keys.has('ArrowRight')?1:0)-(state.keys.has('KeyA')||state.keys.has('ArrowLeft')?1:0);
  const size=Math.hypot(forward,side)||1;
  return {x:(-Math.sin(state.yaw)*forward+Math.cos(state.yaw)*side)/size,z:(-Math.cos(state.yaw)*forward-Math.sin(state.yaw)*side)/size,sprint:state.keys.has('ShiftLeft')||state.keys.has('ShiftRight'),jump:state.keys.has('Space')};
}
function sendMovement(){if(state.connected)send({type:'input',...movement(),yaw:state.yaw,pitch:state.pitch});}
setInterval(sendMovement,50);
function receiveSnapshot(snapshot){
  if(!snapshot.me)return;
  const first=!state.snapshot,old=state.snapshot?.me,oldTraining=state.snapshot?.camelTraining?.exercise,oldExercise=state.snapshot?.cookiePractice?.exercise?.station;
  if(snapshot.layout&&state.layout?.version!==snapshot.layout.version)createEnvironment(snapshot.layout);
  else if(snapshot.layout)state.layout=snapshot.layout;
  state.snapshot=snapshot;state.serverRecipes=snapshot.recipes||[];state.lastSnapshot=performance.now();
  const motionTime=snapshot.time*1000,acceptMotion=motionClock.accept(motionTime,state.lastSnapshot,{reset:snapshot.seq===1});
  if(acceptMotion&&localMotion.push(motionTime,{x:snapshot.me.x,y:(snapshot.me.y||0)+eyeHeight(snapshot.me),z:snapshot.me.z},{key:motionIdentity(snapshot.me),epoch:motionClock.generation}))state.localPosition.set(snapshot.me.x,(snapshot.me.y||0)+eyeHeight(snapshot.me),snapshot.me.z);
  worldGroup.visible=!snapshot.me.island;islandScene?.sync(snapshot);
  raceScene?.sync(snapshot);trainingScene?.sync(snapshot);treeRingScene?.sync(snapshot);practiceScene?.sync(snapshot);townScenery?.update(snapshot);achievementScenery?.sync(snapshot);
  if(state.pendingBakeryEdit&&snapshot.me.editingBakeryFurnitureId===state.pendingBakeryEdit){const object=snapshot.me.bakeryFurniture?.find(o=>o.id===state.pendingBakeryEdit);state.pendingBakeryEdit=null;if(object)beginBuild({object,acquired:true,scope:'bakery'});}
  if(state.pendingBuildEdit&&snapshot.me.editingBuildId===state.pendingBuildEdit){const object=snapshot.island?.objects?.find(o=>o.id===state.pendingBuildEdit);state.pendingBuildEdit=null;if(object)beginBuild({object,acquired:true});}
  if(state.building){const bakery=state.building.scope==='bakery',mode=bakery?snapshot.me.bakeryBuildMode:snapshot.me.buildMode,editing=bakery?snapshot.me.editingBakeryFurnitureId:snapshot.me.editingBuildId;if(!mode||state.building.object&&editing!==state.building.object.id)cancelBuild({notify:false});}
  if(state.pendingBakeryEdit&&!snapshot.me.bakeryBuildMode)state.pendingBakeryEdit=null;
  if(state.building?.scope==='bakery'&&state.building.object){const current=snapshot.me.bakeryFurniture?.find(o=>o.id===state.building.object.id),color=current?.color||snapshot.me.paint?.[current?.kind]||'';if(current&&color!==state.building.color){const rotation=state.building.rotation;beginBuild({object:current,acquired:true,scope:'bakery',silent:true});if(state.building)state.building.rotation=rotation;}}
  if(old&&old.island!==snapshot.me.island){cancelBuild({notify:!activityArena(snapshot.me.island)});state.zone=null;updateHeldCookie();if(activityArena(old.island)||activityArena(snapshot.me.island)){state.yaw=snapshot.me.island==='camel_training'?Math.PI:snapshot.me.yaw||0;state.pitch=snapshot.me.pitch||0;state.keys.clear();if(state.modal)closeModal({resume:false});if(!state.locked)capturePrompt();}}
  if(old?.island==='camel_training'&&snapshot.me.island==='camel_training'&&oldTraining!==snapshot.camelTraining?.exercise){state.yaw=Math.PI;state.pitch=snapshot.me.pitch||0;state.keys.clear();if(state.modal)closeModal({resume:false});if(!state.locked)capturePrompt();}
  if(snapshot.me.island==='cookie_practice'&&oldExercise!==snapshot.cookiePractice?.exercise?.station&&snapshot.cookiePractice?.exercise?.station&&snapshot.cookiePractice.exercise.station!=='practice_pool'){state.yaw=0;state.pitch=snapshot.cookiePractice.exercise.station==='practice_treadmill'?-.22:-.62;state.keys.clear();}
  if(first){state.yaw=snapshot.me.yaw||0;state.pitch=snapshot.me.pitch||0;state.selected=snapshot.me.selected||'sugar';updateHeldCookie();}
  if(old&&snapshot.me.health<old.health-1){$('damage-overlay').style.opacity='.5';setTimeout(()=>$('damage-overlay').style.opacity='0',240);}
  if(old&&!old.sprintExhausted&&snapshot.me.sprintExhausted)toast('Out of stamina — walking to recover. Release Shift, then sprint again once stamina reaches 20%.');
  if(old&&snapshot.me.deaths>old.deaths){toast(cookieArena(snapshot.me.island)?'Crumbled! Back in the ring with a brief shield.':'You were crumbled. All your inventory items are still yours. Back to the Wildwood!','error');}
  if(old&&!old.fallen&&snapshot.me.fallen){state.handSwing=0;toast('You stumbled! Rest for two seconds, then press E, Space, or move to stand up.');}
  state.selectedSlot=snapshot.me.selectedSlot||0;
  if(snapshot.me.selected!==state.selected){state.selected=snapshot.me.selected||'';updateHeldCookie();}
  const avatarSignature=JSON.stringify(snapshot.me.avatar||{});if(state.avatarSignature!==avatarSignature){state.avatarSignature=avatarSignature;const a=snapshot.me.avatar||{};hand.getObjectByName('hand-skin').material=mat(skinTones[a.skin]||skinTones[1]);hand.getObjectByName('hand-sleeve').material=mat(a.shirt==='tank'?(skinTones[a.skin]||skinTones[1]):avatarColor(a.shirtColor,'#688660'));}
  syncObjects((snapshot.nodes||[]).filter(n=>!['bakery_customer','bakery_chef','bakery_cashier','bakery_server'].includes(n.kind)),state.nodes,nodeObject,(o,n)=>{o.visible=n.available!==false;o.userData.node=n;if(n.kind==='camel'){o.userData.motion??=createMotionTrack();o.userData.pose??={};if(acceptMotion&&o.userData.motion.push(motionTime,{...n,y:n.y||0,yaw:n.yaw||0},{key:snapshot.me.island+'|'+!!n.inCave,epoch:motionClock.generation})){o.position.set(n.x,n.y||0,n.z);o.rotation.y=n.yaw||0;}o.visible=o.visible&&!!n.inCave===!!snapshot.me.inCave;}else if(n.kind==='item_drop'||n.kind.startsWith('bakery_')&&n.kind!=='bakery_plot'){o.position.set(n.x,n.y||0,n.z);o.rotation.y=n.yaw||0;o.visible=o.visible&&!!n.inCave===!!snapshot.me.inCave;}if(['wood','cactus'].includes(n.kind)&&scenery.has(n.id)){const root=scenery.get(n.id);root.visible=n.available!==false;sceneryBatches?.setVisible(root,root.visible);}if(n.kind==='bakery_plot')updateBakeryAppearance(n);});
  bakeryFurnishings?.sync(snapshot,scenery);
  bakeryScene?.sync(snapshot,scenery);
  productionScene?.sync(snapshot,scenery);
  nightScene?.sync(snapshot);
  syncObjects(snapshot.players||[],state.players,()=>new THREE.Group(),(o,p)=>{o.userData.player=p;const signature=JSON.stringify([p.callName,p.name,p.avatar,p.friendCode,p.ridingCamel,p.posture,p.heldItem]);if(o.userData.signature!==signature){for(const child of [...o.children]){o.remove(child);disposeObject(child);}const person=makePerson(p.callName||p.name,'#d19b69',false,p.avatar||{},p.heldItem||'');o.add(person);if(p.friendCode){const label=textSprite(p.friendCode,'#e6e9c6','#37544588');person.children.filter(c=>c.isSprite).forEach(c=>c.position.y=p.avatar?.hat?3.05:2.65);label.position.y=p.avatar?.hat?2.65:2.22;label.scale.set(2,.34,1);person.add(label);}if(p.ridingCamel){o.add(islandScene.camel());person.position.y=1.5;}if(p.posture==='sitting')person.scale.y=.65;if(p.posture==='lying'){person.rotation.x=-Math.PI/2;person.position.y=.7;}o.userData.practiceRig=['treadmill','bench_press','leg_press'].includes(p.posture)?createPracticeAvatarRig(THREE,person,p.posture):null;o.userData.signature=signature;}
    o.userData.motion??=createMotionTrack();o.userData.pose??={};
    if(acceptMotion&&o.userData.motion.push(motionTime,p,{key:motionIdentity(p),epoch:motionClock.generation})){o.position.set(p.x,p.y||0,p.z);o.rotation.y=p.yaw||0;}
  });
  syncObjects((snapshot.animals||[]).filter((a)=>a.health>0||a.egg),state.animals,()=>new THREE.Group(),(o,a)=>{if(o.userData.egg!==a.egg||!o.children.length){for(const child of [...o.children]){o.remove(child);disposeObject(child);}const animal=makeAnimal(a);o.add(animal);o.userData.label=animal.userData.label;o.userData.egg=a.egg;o.userData.lastNeed=null;}
    o.userData.motion??=createMotionTrack();o.userData.pose??={};
    if(acceptMotion&&o.userData.motion.push(motionTime,{x:a.x,y:a.y||0,z:a.z,yaw:Number.isFinite(a.heading)?a.heading:o.rotation.y},{key:a.egg?'egg':'animal',epoch:motionClock.generation})){o.position.set(a.x,a.y||0,a.z);o.rotation.y=Number.isFinite(a.heading)?a.heading:o.rotation.y;}
    o.scale.setScalar(a.egg?1:(a.scale||1));o.userData.animal=a;
  });
  syncObjects((snapshot.homes||[]).map(h=>({...h,id:h.owner})),state.homes,()=>new THREE.Group(),(o,h)=>{const signature=JSON.stringify([h.x,h.z,h.kind,h.safe,h.username,h.callName]);if(o.userData.signature!==signature){for(const child of [...o.children]){o.remove(child);disposeObject(child);}o.add(homeObject(h));o.userData.signature=signature;}});
  syncObjects(snapshot.projectiles||[],state.projectiles,(p)=>{const g=makeCookie(p.item,.65);g.position.set(p.x,p.y,p.z);return g;},(o,p)=>{o.userData.target=new THREE.Vector3(p.x,p.y,p.z);o.userData.velocity=new THREE.Vector3(p.vx,p.vy,p.vz);});
  for(const event of snapshot.events||[]){
    if(state.events.has(event.id))continue;state.events.add(event.id);
    if(!first||snapshot.time-event.time<2){addFeed(event.text,event.kind);if((event.kind==='hit'||event.kind==='defeat')&&event.actor===state.player.id){const reward=event.reward>0?' <span class="hit-reward">'+escapeHTML(event.hitZone==='head'?'HEADSHOT':event.hitZone==='hand/foot'?'HAND / FOOT':'BODY')+' +'+event.reward+' coins</span>':'';$('hit-marker').innerHTML='×'+reward;$('hit-marker').hidden=false;clearTimeout(state.hitTimer);state.hitTimer=setTimeout(()=>$('hit-marker').hidden=true,event.reward?900:200);}}
  }
  if(state.events.size>300)state.events=new Set((snapshot.events||[]).map((e)=>e.id));
  const incoming=Object.keys(snapshot.me.friendRequests||{});for(const id of incoming)if(old&&!old.friendRequests?.[id])toast('New friend request. Open Escape → Friends to respond.');
  for(const n of snapshot.nodes||[])if(n.kind==='camel')updateCamelName(state.nodes.get(n.id),n);
  const namesSignature=JSON.stringify([snapshot.me.bakeryName,snapshot.me.camels?.map(c=>[c.id,c.name]),snapshot.me.reputation,snapshot.me.clubResult]);
  if(namesSignature!==state.namesSignature){state.namesSignature=namesSignature;if(state.modal)renderModal();}
  const islandSignature=JSON.stringify([snapshot.island,snapshot.friends,snapshot.me.friendRequests,snapshot.me.publicFriendCode,snapshot.me.homeIsland,snapshot.me.homeOffer,snapshot.me.friendsTeleport,snapshot.me.posture,snapshot.me.buildMode,snapshot.me.editingBuildId,snapshot.me.ridingCamel,snapshot.me.camels?.map(c=>[c.id,c.island,c.rentalUntil]),snapshot.me.camelDiscount,snapshot.me.camelRentalUntil]);
  if(islandSignature!==state.islandSignature){state.islandSignature=islandSignature;if(state.modal)renderModal();updateHUD();}
  const signature=JSON.stringify([snapshot.me.selectedSlot,snapshot.me.selected,snapshot.me.bagSlots,snapshot.me.hotbar,snapshot.me.recovery,snapshot.me.discovered,snapshot.me.coins,snapshot.me.levels,snapshot.me.bakery,snapshot.me.bakeryPlot,snapshot.nodes?.find(n=>n.id===state.context?.id),snapshot.me.bakeryLevel,snapshot.me.bakeryBuildMode,snapshot.me.editingBakeryFurnitureId,snapshot.me.bakeryFurniture,snapshot.me.production,snapshot.me.bakeryDisplay,snapshot.me.bakeryPantry,snapshot.me.bakeryStaff,snapshot.me.bakeryStaffBreaks,snapshot.me.bakeryChefRecipes,snapshot.me.bakeryBusiness?.workers,snapshot.me.bakeryBusiness?.displayCapacity,snapshot.me.bakeryChefRecipe,snapshot.me.bakeryBusiness?.customers,snapshot.me.bakeryBusiness?.chefStatus,snapshot.me.bakeryBusiness?.serverStatus,snapshot.me.achievements,snapshot.me.hungAchievements,snapshot.me.home,snapshot.me.avatar,snapshot.me.callName,snapshot.me.hats,snapshot.me.equipment,snapshot.me.paint,snapshot.me.canBuildHome,snapshot.me.homeSafeReason],(key,value)=>key==='remaining'&&typeof value==='number'?Math.ceil(value):value);
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
  if(!hand)return;const selected=cookieArena(state.snapshot?.me.island)?(state.snapshot.cookieArena?.ammoItem||'sugar'):state.selected;const old=hand.getObjectByName('held-cookie');if(old){hand.remove(old);disposeObject(old);}if(cookieNames[selected]||state.snapshot?.clubCakes?.some(c=>c.id===selected)){const held=makeCookie(cookieNames[selected]?selected:'cake',1);held.name='held-cookie';hand.add(held);}else if(state.snapshot?.town?.treatCatalog?.some(t=>t.id===selected)){const held=createCamelFoodView({THREE,sphere,box,cylinder},selected);held.name='held-cookie';hand.add(held);}else if((state.snapshot?.town?.bouquets||[state.snapshot?.town?.bouquet]).some(b=>b?.id===selected)){const held=createBouquetView({THREE,sphere,box,cylinder},selected);if(held){held.name='held-cookie';hand.add(held);}}else if(isDoughItem(selected)){const held=createDoughView({THREE,sphere,box},selected,1);held.name='held-cookie';hand.add(held);}else if(state.snapshot?.clubDrinks?.some(d=>d.id===selected)){const held=new THREE.Group();held.name='held-cookie';const shake=selected.includes('shake'),color=selected==='chocolate_shake'?'#785138':selected==='strawberry_shake'?'#df9aaf':selected==='orange_juice'?'#efb44c':'#a1c9c6';cylinder(.11,.075,.32,color,0,0,0,held,10);cylinder(.115,.115,.025,'#ecedd9',0,.17,0,held,10);const straw=cylinder(.013,.013,.27,shake?'#e48fa3':'#f2ecd2',.045,.3,0,held,6);straw.rotation.z=-.16;held.position.y=.08;hand.add(held);}
}
function cookieIcon(kind){return '<span class="cookie-icon '+escapeHTML(kind)+'" aria-hidden="true"></span>';}
function itemIcon(kind){return cookieNames[kind]?cookieIcon(kind):'<span class="material-icon" aria-hidden="true">'+(state.snapshot?.clubDrinks?.some(d=>d.id===kind)?'🥤':bakingIngredientIcons[kind]||materialEmoji[kind]||(isDoughItem(kind)?'\u25d5':'◇'))+'</span>';}
function inventory(){return state.snapshot?.me.inventory||{};}
function itemCount(key){return inventory()[key]||0;}
function itemName(key){const ingredient=bakingItemName(key,cookieName);if(ingredient)return ingredient;if(key.startsWith('bakery_')){const [kind,color]=key.slice(7).split(':');return 'Packed '+(state.snapshot?.bakeryFurnitureCatalog?.find(r=>r.id===kind)?.name||pretty(kind))+(color?' · '+pretty(color):'');}return state.snapshot?.town?.flowerCatalog?.find(f=>f.id===key)?.name||state.snapshot?.town?.treatCatalog?.find(t=>t.id===key)?.name||(state.snapshot?.town?.bouquets||[state.snapshot?.town?.bouquet]).find(b=>b?.id===key)?.name||state.snapshot?.clubCakes?.find(c=>c.id===key)?.name||state.snapshot?.clubDrinks?.find(d=>d.id===key)?.name||(cookieNames[key]?cookieName(key):key.startsWith('build_')?pretty(key.slice(6).replace(':',' · ')):pretty(key));}
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
  const clubToggle=$('club-light-toggle');clubToggle.hidden=!nearForestClub(me)&&!cakeEffectActive(me);clubToggle.textContent='L · Flashing lights: '+(clubReducedFlashing()?'reduced':'on');clubToggle.setAttribute('aria-pressed',String(clubReducedFlashing()));
  $('coin-count').textContent=me.coins||0;$('player-name').textContent=me.callName||me.name;
  $('health-value').textContent=Math.ceil(me.health);$('health-bar').style.width=clamp(me.health/me.maxHealth*100,0,100)+'%';
  $('stamina-value').textContent=Math.ceil(me.stamina);$('stamina-bar').style.width=clamp(me.stamina/me.maxStamina*100,0,100)+'%';
  $('stamina-bar').title=me.sprintExhausted?'Walking to recover. Release Shift and recover 20% stamina before sprinting again.':'Hold Shift to sprint';
  $('area-name').textContent=me.island==='cookie_ring'?'COOKIE FIGHTING RING':me.island==='cookie_practice'?'PRACTICE CENTER':me.island==='camel_training'?'TRAINING GARDENS':me.island==='camel_race'?'DUNE DASH':me.island?'HOME ISLAND':me.inCave&&me.x<0?'WILDWOOD UNDERGROUND':areaNames[areaFor(me.x,me.z)];$('position-label').textContent=Math.round(me.x)+' · '+Math.round(me.z);
  const angle=((state.yaw*180/Math.PI)%360+360)%360;$('compass-direction').textContent=['N','NW','W','SW','S','SE','E','NE'][Math.round(angle/45)%8];
  const slots=cookieArena(me.island)?[{item:'sugar',count:'∞'}]:Array.from({length:5},(_,i)=>me.hotbar?.[i]||{item:'',count:0});state.hotkeys=slots.map(s=>s.item);
  const hotbarSignature=JSON.stringify([slots,state.selectedSlot,me.discovered,!!me.homeIsland,me.island,!!insideOwnBakery(state.snapshot)]);
  if(hotbarSignature!==state.hotbarSignature){
    state.hotbarSignature=hotbarSignature;
    $('hotbar').innerHTML=slots.map((slot,i)=>'<button class="hotbar-slot '+((cookieArena(me.island)||i===state.selectedSlot)?'selected ':'')+(!slot.item?'empty':'')+'" data-slot="'+i+'" title="'+escapeHTML(slot.item?itemName(slot.item):'Empty hotbar slot')+' · '+(i+1)+'"><span class="slot-number">'+(i+1)+'</span>'+(slot.item?itemIcon(slot.item)+'<span class="slot-count">'+slot.count+'</span>':'<span class="empty-hotbar">+</span>')+'<span class="slot-safe" aria-label="Kept on death">◆</span></button>').join('')+(!activityArena(me.island)&&(me.homeIsland||me.island||insideOwnBakery(state.snapshot))?'<button class="hotbar-slot" data-slot="5" title="Build menu · 6"><span class="slot-number">6</span><span style="font-size:25px">⌂</span><span class="slot-count">BUILD</span></button>':'');
  }
  const selected=slots[state.selectedSlot]||{item:'',count:0};
  $('selected-cookie-label').innerHTML=selected.item?escapeHTML(itemName(selected.item).toUpperCase())+' <span>'+escapeHTML(cookieNames[selected.item]?cookieNote(selected.item):heldCamelTreat(state.snapshot)?'Press E near your camel to feed it':'Ready in your hotbar')+'</span>':'EMPTY SLOT <span>Move an item here from your inventory</span>';
  if(cookieArena(me.island))$('selected-cookie-label').innerHTML='ARENA COOKIES <span>Unlimited cookies for this area · Left click to throw</span>';
  if(state.building)$('selected-cookie-label').textContent='PLACE '+pretty(state.building.item).toUpperCase()+' · R rotate'+(state.building.scope==='bakery'?' · F pack · T destroy':' · PgUp/PgDn height')+' · click place · Esc cancel';
  if(me.fallen)$('selected-cookie-label').textContent=fallPrompt(me);
  $('buffs').innerHTML=Object.entries(me.buffs||{}).filter(([,seconds])=>seconds>0).map(([key,seconds])=>'<div class="buff"><span>✦</span><strong>'+escapeHTML(buffNames[key]||pretty(key).toUpperCase())+'</strong><span>'+Math.ceil(seconds)+'s</span></div>').join('');
  $('buffs').insertAdjacentHTML('beforeend','<div class="buff" title="A fall roll every 50 metres walked or run. Each alcoholic drink adds 1 percentage point until respawn."><strong>FALL CHANCE</strong><span>'+(me.fallChance??.1).toFixed(1)+'% / 50m</span></div>');
  const need=Math.min(itemCount('wood'),2)+Math.min(itemCount('stick'),2)+Math.min(itemCount('stone'),3);
  if(cookieArena(me.island)){$('objective-title').textContent=me.island==='cookie_ring'?'Cookie showdown':'Practice and grow';$('objective-text').textContent=me.island==='cookie_ring'?'Left click to throw. Ring kills count on both kill boards. R opens standings and the return button.':'Aim at targets or press E at gym equipment. W runs the treadmill, Space presses weights, and swimming laps improves swimming. R for details or return.';$('objective-progress').style.width='100%';}
  else if(me.island==='camel_training'){$('objective-title').textContent='Practice makes progress';$('objective-text').textContent=state.snapshot.camelTraining?.courses?.find(c=>c.id===state.snapshot.camelTraining.exercise)?.description||'Ride the course through each golden gate. E or R chooses an exercise or returns to Camel Cove.';$('objective-progress').style.width=Math.min(100,state.snapshot.camelTraining?.ownCamel?.training?.[state.snapshot.camelTraining.exercise]||0)+'%';}
  else if(me.island==='camel_race'){$('objective-title').textContent='Dune Dash';$('objective-text').textContent='Follow the arrows and golden gates for three laps. Shift sprints, Space jumps. E or R shows standings and the return button.';$('objective-progress').style.width=(Math.min(3,state.snapshot.camelRace?.entrants?.find(e=>e.id===me.id)?.lap||0)/3*100)+'%';}
  else if(me.inCave&&me.x<0){$('objective-title').textContent='Wildwood after dark';$('objective-text').textContent='Press E at the bar, wheel, or a chair. At night, find Disco Duke by the far end of the dance floor for funny cakes. L reduces light and cake effects.';$('objective-progress').style.width='100%';}
  else if(me.bakeryBuildMode){$('objective-title').textContent='Decorate your bakery';$('objective-text').textContent='6 opens furnishings. E selects, R rotates, T destroys, F packs, left click places. B exits build mode.';$('objective-progress').style.width='100%';}
  else if(me.buildMode){$('objective-title').textContent='Build mode';$('objective-text').textContent='6 opens the build catalog. Look at an object + E to edit. R rotates, T destroys, F packs, left click places, Esc cancels.';$('objective-progress').style.width='100%';}
  else if(me.island){$('objective-title').textContent='A place of your own';$('objective-text').textContent='Press 6 to build and decorate. Open Friends to invite visitors and choose who can help build.';$('objective-progress').style.width='100%';}
  else if(me.homeIsland){$('objective-title').textContent='Home across the water';$('objective-text').textContent='Press 6 to return home. Gather wood and stones, or find a wounded fish or turtle for your pond.';$('objective-progress').style.width='100%';}
  else if(me.coins>=200){$('objective-title').textContent='A sweet path home';$('objective-text').textContent='Find the cave near the middle of Sunbaked Sands. The pink lollipop offers your first island for 200 coins.';$('objective-progress').style.width='100%';}
  else if(me.bakery){$('objective-title').textContent=me.bakeryPlot?'The next big batch':'Reopen your bakery';$('objective-text').textContent=me.bakeryPlot?'Stock your display, serve visiting customers, or hire a chef and cashier.':'Claim any available city bakery plot for free to restore your saved bakery.';$('objective-progress').style.width='100%';}
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
function chooseCookie(key){if(activityArena(state.snapshot?.me.island))return;const slot=typeof key==='number'?key:(state.snapshot?.me.hotbar||[]).findIndex(s=>s.item===key);if(slot===5){if(insideOwnBakery(state.snapshot)){openModal('bakery_build');return;}if(state.snapshot?.me.homeIsland||state.snapshot?.me.island){openModal('build');return;}}if(slot<0||slot>4)return;cancelBuild();if(state.snapshot?.me.bakeryBuildMode)action('bakery_build_mode',{enabled:false});if(state.snapshot?.me.buildMode)action('build_mode',{enabled:false});state.selectedSlot=slot;state.selected=state.snapshot?.me.hotbar?.[slot]?.item||'';action('select',{slot});updateHeldCookie();updateHUD();}
function toggleBakeryBuild(){if(!insideOwnBakery(state.snapshot)&&!state.snapshot?.me.bakeryBuildMode){toast('Step inside your own bakery to redecorate.');return;}cancelBuild();action('bakery_build_mode',{enabled:!state.snapshot.me.bakeryBuildMode});}
$('hotbar').addEventListener('click',(event)=>{const button=event.target.closest('[data-slot]');if(button)chooseCookie(Number(button.dataset.slot));});
function toggleClubLighting(){setClubReducedFlashing(!clubReducedFlashing());updateHUD();if(state.modal)renderModal();}
$('club-light-toggle').addEventListener('click',toggleClubLighting);
function throwCookie(){
  if(camelArena(state.snapshot?.me.island))return;
  if(document.pointerLockElement!==$('world')||state.modal||!state.snapshot||performance.now()-state.lastThrow<160)return;
  if(state.snapshot.me.fallen){toast(fallPrompt(state.snapshot.me));return;}
  if(state.building){placeBuild();return;}
  if(state.snapshot.me.buildMode||state.snapshot.me.bakeryBuildMode){toast('Press 6 to choose a build, or look at an object and press E to edit.');return;}
  const customer=bakeryScene?.pickCustomer(camera,state.snapshot,4);if(customer){openModal('bakery_customer',customer);return;}
  state.lastThrow=performance.now();send({type:'input',...movement(),yaw:state.yaw,pitch:state.pitch});action('throw',{item:cookieArena(state.snapshot.me.island)?'sugar':state.selected});state.handSwing=1;
}
function eatCookie(){if(!state.snapshot||!state.selected||activityArena(state.snapshot.me.island))return;if(state.snapshot.me.fallen){toast(fallPrompt(state.snapshot.me));return;}action(state.snapshot.clubDrinks?.some(d=>d.id===state.selected)?'drink':['protein_powder','protein_drink'].includes(state.selected)?'consume_protein':'eat',{item:state.selected});state.handSwing=-1;}
function nearby(){
  const me=state.snapshot?.me;if(!me)return null;const feedingTarget=heldTreatTarget(state.snapshot);if(feedingTarget)return feedingTarget;let found=null,best=5;
  for(const node of state.snapshot.nodes||[]){if(['tree_golden_cookie','tree_silver_cookie'].includes(node.kind)&&(Math.abs((node.y||0)-(me.y||0))>2||Math.hypot(node.x-me.x,node.z-me.z)>3.2))continue;if(!node.available||node.kind==='camel'&&(node.owner!==me.id||me.ridingCamel||Math.abs((node.y||0)-(me.y||0))>2))continue;if(node.kind==='item_drop'&&Math.abs((node.y||0)-(me.y||0))>2)continue;const underground=!!node.inCave||['pink_lollipop','blue_lollipop'].includes(node.kind);if(underground&&!me.inCave||!underground&&me.inCave)continue;const d=node.kind==='bakery_plot'?Math.min(Math.hypot(node.x-me.x,node.z-me.z),Math.hypot(node.x-2.7-me.x,node.z+8.6-me.z)):Math.hypot(node.x-me.x,node.z-me.z);if((node.kind==='club_seat'||node.kind==='bakery_chair')&&d>2.5||node.kind==='island_portal'&&d>3||['bakery_oven','bakery_mixer','bakery_cooling_table','bakery_display','bakery_fridge'].includes(node.kind)&&d>2.6)continue;if(d<best){best=d;found={...node,distance:d,isAnimal:false};}}
  for(const animal of state.snapshot.animals||[]){if(animal.health<=0&&!animal.egg)continue;const d=Math.hypot(animal.x-me.x,animal.z-me.z);if(d<best){best=d;found={...animal,distance:d,isAnimal:true};}}
  for(const object of state.snapshot.island?.objects||[]){const d=Math.hypot(object.x-me.x,object.z-me.z);if(d<best&&Math.abs((object.y||0)-(me.y||0))<3){best=d;found={...object,distance:d,isBuild:true};}}
  return found;
}
function updateInteraction(){
  const exercise=state.snapshot?.cookiePractice?.exercise;if(exercise?.station&&exercise.station!=='practice_pool'){state.nearest=null;$('interaction-prompt').hidden=!state.locked;$('interaction-label').textContent=exercise.status||'Keep training';$('interaction-detail').textContent=exercise.station==='practice_treadmill'?'Hold W to run · E or Q to step off':'Press Space for a repetition · E or Q to stop';return;}
  if(state.snapshot?.me.island==='camel_training'){state.nearest=null;$('interaction-prompt').hidden=!state.locked;$('interaction-label').textContent='Training courses';$('interaction-detail').textContent='E or R \u00b7 Exercises, treats and return to Camel Cove';return;}
  if(state.snapshot?.me.island==='camel_race'){state.nearest=null;$('interaction-prompt').hidden=!state.locked;$('interaction-label').textContent='Race standings';$('interaction-detail').textContent='E or R · View the race / return to the beach';return;}
  if(state.snapshot?.me.fallen){state.nearest=null;$('interaction-prompt').hidden=!state.locked;$('interaction-label').textContent=state.snapshot.me.standIn>0?'You fell over':'Stand up';$('interaction-detail').textContent=fallPrompt(state.snapshot.me);return;}
  if(state.snapshot?.me.bakeryBuildMode&&!state.building){const hit=bakeryFurnishings?.pickObject(camera,state.snapshot.me.bakeryPlot,12);state.nearest=hit?{...hit,isBakeryFurniture:true}:null;}
  else if(state.snapshot?.me.buildMode&&!state.building){const hit=islandScene?.pickObject(camera,12),object=state.snapshot.island?.objects?.find(o=>o.id===hit?.id);state.nearest=object?{...object,isBuild:true,distance:hit.distance}:null;}
  else state.nearest=nearby();const n=state.nearest;$('interaction-prompt').hidden=!n||!state.locked;
  if(!n)return;
  if(['cookie_ring_exit','cookie_practice_exit'].includes(n.kind)){$('interaction-label').textContent='Return to the treehut';$('interaction-detail').textContent='E to go back to the southern tree';return;}
  if(n.kind.startsWith('practice_')){$('interaction-label').textContent=n.label||'Practice';$('interaction-detail').textContent=n.kind==='practice_pool'?'Swim to the far end and back in one lane':'E to use this equipment';return;}
  if(n.kind==='tree_golden_cookie'||n.kind==='tree_silver_cookie'){$('interaction-label').textContent=n.kind==='tree_golden_cookie'?'Eat the golden cookie':'Eat the silver cookie';$('interaction-detail').textContent=n.kind==='tree_golden_cookie'?'Enter the cookie fighting ring':'Visit aim practice and the physical gym';return;}
  if(n.kind==='camel_training_lollipop'){$('interaction-label').textContent='Taste the pink camel lollipop';$('interaction-detail').textContent='Train your camel \u00b7 Treats speed up practice';return;}
  if(n.kind==='camel_race_lollipop'){$('interaction-label').textContent='Taste the camel lollipop';$('interaction-detail').textContent='Enter the next Dune Dash · Every 18 minutes';return;}
  if(n.kind==='flower_shop'||n.kind==='betty'){$('interaction-label').textContent=n.kind==='betty'?'Have a chat with Betty':'Visit Betty’s flower shop';$('interaction-detail').textContent='Sell single flowers · Buy a bouquet · Make someone smile';return;}
  if(n.kind==='pet_shop'){$('interaction-label').textContent='Visit the pet store';$('interaction-detail').textContent='Treats for speed, jumping, stamina, and happiness';return;}
  if(n.kind==='leaderboard_board'){$('interaction-label').textContent=n.label;$('interaction-detail').textContent='Four leaderboards · Your place in Crumb City';return;}
  if(n.kind.startsWith('flower_')){$('interaction-label').textContent='Pick '+itemName(n.kind);$('interaction-detail').textContent='Sell a single flower to Betty for coins';return;}
  if(n.isBakeryFurniture){$('interaction-label').textContent='Edit '+pretty(n.kind).toLowerCase();$('interaction-detail').textContent='E selects - R rotates - T destroys - F packs';return;}
  if(n.kind==='bakery_chair'){$('interaction-label').textContent=state.snapshot.me.bakerySeatId===n.id?'Stand up':'Sit in cafe chair';$('interaction-detail').textContent=n.occupiedBy&&n.occupiedBy!==state.snapshot.me.id?'This chair is occupied':'Relax in the cafe';return;}
  if(n.kind==='club_cake_seller'){$('interaction-label').textContent='Buy funny cakes';$('interaction-detail').textContent='Disco Duke · Strange colors and a wandering world';return;}
  if(n.kind.startsWith('club_')){$('interaction-label').textContent=n.kind==='club_bar'?'Order a drink':n.kind==='club_casino'?'Play the fortune wheel':state.snapshot.me.posture==='sitting'?'Stand up':'Take a seat';$('interaction-detail').textContent=n.kind==='club_bar'?'Shakes, soft drinks, and bar favorites':n.kind==='club_casino'?'Choose your stake in game coins · View all seven outcomes':'Relax in the Wildwood underground';return;}
  if(n.kind==='item_drop'){$('interaction-label').textContent='Pick up '+itemName(n.item)+' ('+(n.amount||1)+')';$('interaction-detail').textContent='E to collect this item or bundle';return;}
  if(state.building){$('interaction-prompt').hidden=true;return;}
  if(state.snapshot?.me.buildMode&&n.isBuild){$('interaction-label').textContent='Edit '+pretty(n.kind).toLowerCase();$('interaction-detail').textContent=n.editingBy&&n.editingBy!==state.snapshot.me.id?'Another builder is editing this object':'E takes control · R rotates · T destroys · F packs';return;}
  if(n.kind==='island_upgrade'){$('interaction-label').textContent='Expand your home island';$('interaction-detail').textContent='E to view prices \u00b7 Up to five expansions';return;}
  if(n.kind==='island_portal'){$('interaction-label').textContent='Teleport to the clay cave';$('interaction-detail').textContent='Return to the pink and blue lollipop chambers';return;}
  if(n.kind==='camel'&&heldCamelTreat(state.snapshot)){const treat=heldCamelTreat(state.snapshot);$('interaction-label').textContent='Feed '+(n.label||'your camel');$('interaction-detail').textContent='E gives one '+treat.name+' \u00b7 2x '+treat.stat+' training';return;}
  if(n.kind==='camel'){$('interaction-label').textContent='Mount your camel';$('interaction-detail').textContent='E to mount · Q to dismount · Rides with you when you teleport';return;}
  if(n.kind==='ingredient_shop'){$('interaction-label').textContent='Buy baking ingredients';$('interaction-detail').textContent='Sugar, butter, flour, and eggs for your mixer';return;}
  if(n.isBuild&&n.kind==='table'&&state.snapshot.me.production?.['home:'+n.id]){$('interaction-label').textContent='Collect cooled bakes';$('interaction-detail').textContent='Finished goods from your home oven';return;}
  if(n.isBuild&&['oven','mixer'].includes(n.kind)){$('interaction-label').textContent=n.kind==='oven'?'Bake cookies':'Mix cookie dough';$('interaction-detail').textContent='Prepare your next batch';return;}
  if(['bakery_oven','bakery_mixer','bakery_cooling_table','bakery_display','bakery_fridge'].includes(n.kind)){$('interaction-label').textContent={bakery_fridge:'Stock the chef fridge',bakery_oven:'Bake cookies',bakery_mixer:'Mix dough & recipe ingredients',bakery_cooling_table:'Collect finished bakes',bakery_display:'Stock display & serve customers'}[n.kind];$('interaction-detail').textContent=n.owner===state.snapshot?.me.id?'Your bakery station':'This station belongs to another baker';return;}
  if(n.isBuild){$('interaction-label').textContent=n.kind==='storage_chest'?'Open personal storage':n.kind==='chest'?'Open shared materials chest':'Use '+pretty(n.kind).toLowerCase();$('interaction-detail').textContent=n.kind==='storage_chest'?'Private storage for all your items':n.kind==='chest'?'Visitors may donate · Only the owner may withdraw':'Enter build mode to move, rotate, pack or destroy';return;}
  if(n.egg){$('interaction-label').textContent='A '+pretty(n.species).toLowerCase()+' egg';$('interaction-detail').textContent='Keep your distance · '+Math.ceil(n.hatchIn||120)+' seconds undisturbed to hatch';return;}
  if(['wood','cactus'].includes(n.kind)){$('interaction-label').textContent=(n.kind==='wood'?'Chop tree':'Chop cactus')+' · '+(n.chopRemaining||n.chopTotal||3)+' chops left';$('interaction-detail').textContent='Collect '+(n.amount||3)+' '+n.kind+' when it falls';return;}
  const phrases={wood:['Gather wood','Campfires start with the little things'],stick:['Gather sticks','Every good batch starts somewhere'],stone:['Collect stones','For fire pits and fair trades'],berry:['Pick wild berries','Try them in your next recipe'],nut:['Gather nuts','Something new for the cookbook'],cactus:['Harvest cactus','An ingredient from the sunbaked sands'],sea_salt:['Collect sea salt','A taste of the ocean'],shell:['Collect seashells','A little treasure from the tide'],trash:['Clean up the beach','Peace pays 3 coins per kilogram'],chest:['Open chest','Good things come in wooden boxes'],fire:['Bake at the campfire','Turn your finds into something delicious'],peace:['Trade with Peace','Good karma. Fair prices.'],desert_trader:['Visit Saffron','The desert has a few secret recipes'],gym:['Get a little stronger','Good cookies deserve a good throwing arm'],vending:['Use protein vending machine','Fuel your next workout or next batch'],land:['Buy bakery land','Build your little cookie empire'],bakery:['Manage your bakery','Fresh batches and brighter prospects'],dummy:['Practice your aim','Left click to throw a cookie']};
  if(n.isAnimal){const reputation=state.snapshot.me.reputation?.[n.species]||0;$('interaction-label').textContent=n.need==='trapped'?'Free the '+pretty(n.species).toLowerCase():n.need==='wounded'?'Help the wounded '+pretty(n.species).toLowerCase():reputation>0?'Greet the '+pretty(n.species).toLowerCase():pretty(n.species);$('interaction-detail').textContent=n.need==='wounded'?'Offer one '+itemName('sugar').toLowerCase()+' to help it heal':n.need==='trapped'?'Helping improves your reputation with this species':reputation<0?'This species distrusts you · Reputation '+reputation:reputation>0?'This species likes you · Reputation +'+reputation:'This species is neutral · Help animals in need to earn trust';}
  else{const p=phrases[n.kind]||[n.label,'Press E to interact'];$('interaction-label').textContent=p[0];$('interaction-detail').textContent=p[1];}
}
function interact(){
  if(state.snapshot?.cookiePractice?.exercise?.station&&state.snapshot.cookiePractice.exercise.station!=='practice_pool'){action('practice_stop');return;}
  if(state.snapshot?.me.island==='camel_training'){openModal('training');return;}
  if(state.snapshot?.me.island==='camel_race'){openModal('race');return;}
  if(state.snapshot?.me.fallen){if(state.snapshot.me.standIn>0)toast(fallPrompt(state.snapshot.me));else action('stand');return;}
  if(state.snapshot?.me.bakeryBuildMode){if(state.building){toast('Place or cancel this furnishing first.');return;}const hit=bakeryFurnishings?.pickObject(camera,state.snapshot.me.bakeryPlot,12);if(!hit){toast('Look at a bakery furnishing within 12 metres to edit it.');return;}state.pendingBakeryEdit=hit.id;action('bakery_edit',{target:hit.id});return;}
  if(state.snapshot?.me.buildMode){if(state.building){toast('Place or cancel the object you are holding first.');return;}const hit=islandScene?.pickObject(camera,12);if(!hit){toast('Look at an island object within 12 metres to edit it.');return;}state.pendingBuildEdit=hit.id;action('edit_build',{target:hit.id});return;}
  const n=state.nearest||nearby();if(!n){toast('Move closer to something you can gather or use.');return;}
  if(n.kind==='club_bar'||n.kind==='club_casino'||n.kind==='club_cake_seller'){openModal('context',n);return;}
  if(n.kind==='bakery_chair'){action('bakery_sit',{target:n.id});return;}
  if(n.kind==='club_seat'){action('club_sit',{target:n.id});return;}
  if(n.kind==='item_drop'){action('pickup_item',{target:n.id});return;}
  if(n.isBuild){openModal(n.kind==='chest'?'chest':n.kind==='storage_chest'?'storage':['oven','mixer','table'].includes(n.kind)?'context':'build_object',n);return;}
  if(n.kind==='island_upgrade'){openModal('context',n);return;}
  if(n.kind==='island_chest'){openModal('chest',n);return;}
  if(n.kind==='island_portal'){cancelBuild();action('teleport_cave');return;}
  if(['cookie_ring_exit','cookie_practice_exit'].includes(n.kind)){action('cookie_arena_leave');return;}
  if(n.kind.startsWith('practice_')){if(n.kind!=='practice_target')action('practice_exercise',{target:n.id});return;}
  if(n.kind==='camel'){action('interact',{target:n.id});if(heldCamelTreat(state.snapshot))state.handSwing=.8;return;}
  if(n.egg){toast('Stay at least two metres away so this egg can hatch undisturbed.');return;}
  if(n.isAnimal&&n.need==='wounded'&&['fish','turtle'].includes(n.species)){openModal('animal',n);return;}
  if(n.isAnimal){action('interact',{target:n.id});return;}
  if(n.kind==='fire'){openModal('recipes',n);return;}if(n.kind==='outfit_shop'){openModal('customize',n);return;}
  if(['tree_golden_cookie','tree_silver_cookie','peace','desert_trader','gym','land','bakery','bakery_plot','bakery_oven','bakery_mixer','bakery_cooling_table','bakery_display','bakery_fridge','ingredient_shop','kitchen_shop','paint_shop','outfit_shop','pink_lollipop','blue_lollipop','abu_fanous','camel_training_lollipop','camel_race_lollipop','leaderboard_board','flower_shop','pet_shop','betty'].includes(n.kind)){openModal('context',n);return;}
  action('interact',{target:n.id});
}
function cancelBuild({notify=true}={}){
  if(notify&&(state.building?.object||state.pendingBuildEdit||state.pendingBakeryEdit))action(state.building?.scope==='bakery'||state.pendingBakeryEdit?'bakery_cancel_edit':'cancel_build');state.pendingBuildEdit=null;state.pendingBakeryEdit=null;state.buildOperationPending=false;islandScene?.setEditing('');bakeryFurnishings?.setEditing('');
  state.building=null;if(buildGhost){scene.remove(buildGhost);buildGhost.traverse(o=>{if(o.isMesh)o.material.dispose();});disposeObject(buildGhost);buildGhost=null;}
}
function beginBuild({item,object=null,packed='',acquired=false,scope='island',silent=false}){
  const bakery=scope==='bakery',me=state.snapshot?.me;if(bakery?!me?.bakeryBuildMode||!insideOwnBakery(state.snapshot):!me?.island||!me.buildMode){toast(bakery?'Enter bakery build mode first.':'Enter build mode from the home island menu first.');return;}
  cancelBuild({notify:!acquired});state.building={scope,item:item||object?.kind,object,packed,rotation:object?.rotation||0,y:bakery?0:object?.y||0};
  const definition={...object,kind:state.building.item,pondCapacity:object?.pondCapacity||Number(packed.match(/^build_pond:capacity([2-4])$/)?.[1])||1,color:object?.color||packed.split(':')[1]||(bakery?me.paint?.[state.building.item]:'')||'',lit:true};state.building.color=definition.color;
  buildGhost=bakery?bakeryFurnishings.objectView(definition,state.snapshot.bakeryFurnitureCatalog):islandScene.objectView(definition,state.snapshot.buildCatalog);
  (bakery?bakeryFurnishings:islandScene).setEditing(object?.id||'');
  buildGhost.traverse(o=>{if(o.isMesh){o.material=o.material.clone();o.userData.ghostColor=o.material.color.clone();o.material.transparent=true;o.material.opacity=.48;o.material.depthWrite=false;o.castShadow=false;}});scene.add(buildGhost);
  if(!silent){toast('R rotates - T destroys - F packs - Left click places - Esc cancels.');if(!state.locked&&!state.modal)setPlaying();}
}
function updateBuildGhost(){
  if(!state.building||!buildGhost||!state.snapshot)return;
  const b=state.building,me=state.snapshot.me,range=clamp((1.7+(me.y||0)-b.y)/Math.max(.2,Math.tan(-state.pitch)),2.5,7),plot=b.scope==='bakery'?ownedBakery(state.snapshot):null;
  const wx=Math.round((me.x-Math.sin(state.yaw)*range)*2)/2,wz=Math.round((me.z-Math.cos(state.yaw)*range)*2)/2;b.x=wx-(plot?.x||0);b.z=wz-(plot?.z||0);
  buildGhost.position.set(wx,b.y,wz);buildGhost.rotation.y=b.rotation*Math.PI/180;
  const valid=plot?bakeryPlacementValid({id:b.object?.id,kind:b.item,x:b.x,z:b.z,rotation:b.rotation},state.snapshot.bakeryFurnitureCatalog,me.bakeryFurniture):Math.hypot(b.x,b.z)<(state.snapshot.island?.buildRadius||32);b.valid=valid;buildGhost.traverse(o=>{if(o.isMesh)o.material.color.copy(o.userData.ghostColor).lerp(valid?buildGhostTints.valid:buildGhostTints.invalid,.4);});
  buildGhost.visible=state.locked&&!state.modal;
}
function placeBuild(){
  const b=state.building;if(!b||state.buildOperationPending)return;
  if(b.scope==='bakery'&&!b.valid){toast('Keep furnishings inside the right room, away from doors and other furniture.');return;}
  state.buildOperationPending=!!b.object;
  action(b.scope==='bakery'?(b.object?'bakery_move':'bakery_place'):(b.object?'move_build':'build'),{item:b.item,from:b.packed||'',target:b.object?.id||'',x:b.x,y:b.y,z:b.z,rotation:b.rotation});state.lastThrow=performance.now();
  if(!b.object)cancelBuild({notify:false});
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
function refreshAmbientControls(){const muted=ambientAudio.muted;$('ambient-toggle').textContent='World sounds '+(muted?'off':'on');$('ambient-toggle').setAttribute('aria-pressed',String(muted));$('ambient-volume').value=String(Math.round(ambientAudio.volume*100));$('ambient-volume-label').textContent=Math.round(ambientAudio.volume*100)+'%';}
$('ambient-toggle').addEventListener('click',()=>{ambientAudio.setMuted(!ambientAudio.muted);ambientAudio.unlock();refreshAmbientControls();});
$('ambient-volume').addEventListener('input',event=>{ambientAudio.setVolume(Number(event.target.value)/100);refreshAmbientControls();});
refreshAmbientControls();
window.addEventListener('pagehide',()=>ambientAudio.clear());
let escapeHandled=false;
$('play-button').addEventListener('click',setPlaying);
$('start-button').addEventListener('click',setPlaying);
$('capture-prompt').addEventListener('click',setPlaying);
$('customize-button').addEventListener('click',()=>openModal('customize'));
$('world').addEventListener('click',()=>{if(state.player&&!state.locked&&!state.modal&&!state.menuOpen)setPlaying();});
document.addEventListener('pointerlockchange',()=>{
  state.locked=document.pointerLockElement===$('world');state.playing=state.locked;
  state.keys.clear();sendMovement();
  if(state.locked){state.hasPlayed=true;state.menuOpen=false;$('play-overlay').hidden=true;$('start-overlay').hidden=true;$('capture-prompt').hidden=true;}
  else capturePrompt();
});
document.addEventListener('pointerlockerror',capturePrompt);
document.addEventListener('mousemove',(event)=>{if(!state.locked)return;state.yaw-=event.movementX*.002;state.pitch=clamp(state.pitch-event.movementY*.002,-1.35,1.35);});
document.addEventListener('pointerdown',()=>{nightScene?.unlockAudio();ambientAudio.unlock();});
document.addEventListener('mousedown',(event)=>{if(document.pointerLockElement!==$('world')||state.modal)return;if(event.button===0)throwCookie();if(event.button===2)eatCookie();});
document.addEventListener('contextmenu',(event)=>{if(state.player)event.preventDefault();});
const movementKeys=new Set(['KeyW','KeyA','KeyS','KeyD','ArrowUp','ArrowDown','ArrowLeft','ArrowRight','Space','ShiftLeft','ShiftRight']);
document.addEventListener('keydown',(event)=>{
  nightScene?.unlockAudio();ambientAudio.unlock();
  const pointerLocked=document.pointerLockElement===$('world');
  if(event.code==='Escape'){if(event.repeat)return;event.preventDefault();escapeHandled=true;if(state.building){cancelBuild();return;}toggleMenu();return;}
  if(event.target instanceof HTMLInputElement||event.target instanceof HTMLTextAreaElement||event.target instanceof HTMLSelectElement||event.ctrlKey||event.metaKey||event.altKey)return;
  if(!state.player)return;
  if(state.locked&&movementKeys.has(event.code)){event.preventDefault();state.keys.add(event.code);}
  if(event.repeat)return;
  if(state.locked&&movementKeys.has(event.code)){sendMovement();return;}
  if(state.building&&pointerLocked&&!state.modal){if(['KeyT','KeyF'].includes(event.code)&&state.building.object&&!state.buildOperationPending){event.preventDefault();state.buildOperationPending=true;action(state.building.scope==='bakery'?(event.code==='KeyT'?'bakery_destroy':'bakery_pack'):(event.code==='KeyT'?'destroy_build':'pack_build'),{target:state.building.object.id});return;}if(event.code==='KeyR'){state.building.rotation=(state.building.rotation+45)%360;return;}if(state.building.scope!=='bakery'&&(event.code==='PageUp'||event.code==='PageDown')){event.preventDefault();state.building.y=clamp(state.building.y+(event.code==='PageUp'?.5:-.5),0,12);return;}}
  if(event.code==='Enter'&&!state.locked&&!state.modal)setPlaying();
  if(state.menuOpen)return;
  if(event.code==='KeyB'&&!state.modal){event.preventDefault();toggleBakeryBuild();return;}
  if(event.code==='KeyL'&&(nearForestClub(state.snapshot?.me)||cakeEffectActive(state.snapshot?.me))){event.preventDefault();toggleClubLighting();return;}
  if(event.code==='KeyI')toggleModal('inventory');
  if(event.code==='KeyC')toggleModal('recipes');
  if(event.code==='KeyM')toggleModal('map');
  if(event.code==='KeyR'&&cookieArena(state.snapshot?.me.island))toggleModal('cookie_activity');
  if(event.code==='KeyR'&&camelArena(state.snapshot?.me.island))toggleModal(state.snapshot.me.island==='camel_training'?'training':'race');
  if(event.code==='KeyE'&&state.locked)interact();
  if(event.code==='KeyQ'&&state.locked){if(state.snapshot?.cookiePractice?.exercise?.station)action('practice_stop');else if(cookieArena(state.snapshot?.me.island))openModal('cookie_activity');else if(camelArena(state.snapshot?.me.island))openModal(state.snapshot.me.island==='camel_training'?'training':'race');else if(state.snapshot?.me.ridingCamel)action('camel_dismount');else eatCookie();}
  if(/^Digit[1-6]$/.test(event.code)&&!state.modal)chooseCookie(Number(event.code.slice(5))-1);
});
document.addEventListener('keyup',(event)=>{const held=state.keys.delete(event.code);if(held&&movementKeys.has(event.code))sendMovement();if(event.code==='Escape'){if(!escapeHandled)toggleMenu();escapeHandled=false;}});
window.addEventListener('blur',()=>{state.keys.clear();escapeHandled=false;sendMovement();});
document.addEventListener('visibilitychange',()=>{if(document.hidden){state.keys.clear();sendMovement();ambientAudio.clear();}});
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
  const underground=me?.inCave&&!me.island?clamp(-(me.y||0)/7,0,1):0;
  const inClub=!!me?.inCave&&!me.island&&me.x<0;
  if(daylight){const night=nightFactor(state.snapshot?.dayCycle);daylight.intensity=(3.25*(1-night)+.2*night)*(1-underground)+.04*underground;daylight.color.set('#fff1cf').lerp(new THREE.Color('#a7bce8'),night);skylight.intensity=(2.15*(1-night)+.5*night)*(1-underground)+(inClub?1.05:.13)*underground;scene.fog.color.set('#a9c1a5').lerp(new THREE.Color('#17283d'),night).lerp(new THREE.Color(inClub?'#101224':'#261307'),underground);scene.fog.near=75-63*underground;scene.fog.far=235-185*underground;scene.background.copy(new THREE.Color('#b7cbb8')).lerp(new THREE.Color('#101d35'),night).lerp(new THREE.Color(inClub?'#090c19':'#1b0c05'),underground);}
  for(const light of caveScenery?.userData.lights||[])light.visible=!!me?.inCave&&!me.island&&!inClub;
  forestClub?.update(now/1000,inClub||nearForestClub(me),me?.clubResult,state.connected&&state.snapshot?.dayCycle?.phase==='night');
  funnyCakeEffects?.update({remaining:Math.max(0,(me?.buffs?.funny_cake||0)-(now-state.lastSnapshot)/1000),time:now/1000,reduced:clubReducedFlashing(),enabled:state.connected&&!!state.player&&!!me});
  bakeryScene?.update(now/1000,dt);
  productionScene?.update(now/1000);
  nightScene?.update(now/1000,dt);
  raceHUD?.update();trainingHUD?.update();trainingScene?.update(now/1000);treeRingScene?.update(now/1000);practiceScene?.update(now/1000);cookieActivityHUD?.update();
  if(state.player&&me){
    if(localMotion.sample(motionClock.time(now,LOCAL_MOTION_DELAY_MS),localPose))state.localPosition.set(localPose.x,localPose.y,localPose.z);
    const moving=movement();const walking=!me.fallen&&Math.hypot(moving.x,moving.z)>.05;
    const bob=walking&&state.locked&&me.grounded!==false?Math.sin(now*.012*(me.sprinting?1.45:1))*.027:0;
    camera.position.copy(state.localPosition);camera.position.y+=bob;camera.rotation.set(state.pitch,state.yaw,0);
    riddenCamel.visible=!!me.ridingCamel;if(riddenCamel.visible){riddenCamel.position.set(state.localPosition.x,state.localPosition.y-eyeHeight(me),state.localPosition.z);riddenCamel.rotation.y=state.yaw;animateCamelView(riddenCamel,{time:now/1000,moving:walking,eating:(me.camels?.find(c=>c.id===me.mountedCamelId)?.eatingUntil||0)>state.snapshot.time});}
    camera.fov=THREE.MathUtils.lerp(camera.fov,me.sprinting&&walking?73:68,dt*5);camera.updateProjectionMatrix();
    if(me.island)islandScene?.animate(now);
    updateBuildGhost();
    hand.visible=!me.fallen&&!state.building&&!camelArena(me.island)&&!['treadmill','bench_press','leg_press'].includes(me.posture);hand.position.set(.48,-.43+bob*.55,-.82);
    if(state.handSwing>0){hand.position.z-=Math.sin((1-state.handSwing)*Math.PI)*.36;hand.rotation.x=.55-state.handSwing*.35;state.handSwing=Math.max(0,state.handSwing-dt*4);}
    else if(state.handSwing<0){hand.position.y+=Math.sin((1+state.handSwing)*Math.PI)*.33;hand.position.x-=Math.sin((1+state.handSwing)*Math.PI)*.3;state.handSwing=Math.min(0,state.handSwing+dt*2.5);}
    else hand.rotation.x=.55;
    hand.rotation.z=-.24+(walking?Math.sin(now*.006)*.025:Math.sin(now*.002)*.009);
    if(now-lastUI>120){updateHUD();updateInteraction();lastUI=now;}
    if(now-lastPanelRefresh>1000&&(['cookie_activity','customize','build_object','chest','animal','race','training','achievements','inventory','friends'].includes(state.modal)||(state.modal==='shop'&&['tree_golden_cookie','tree_silver_cookie','outfit_shop','abu_fanous','camel_training_lollipop','camel_race_lollipop','leaderboard_board','flower_shop','pet_shop','betty'].includes(state.context?.kind)))){renderModal();lastPanelRefresh=now;}
    if(now-lastMinimap>150){panels?.drawMinimap();if(state.modal==='map')panels?.drawWorldMap();lastMinimap=now;}
    if(interactionRing){interactionRing.visible=!!state.nearest&&state.locked&&!state.building;if(state.nearest){interactionRing.position.set(state.nearest.x,(me.y||0)+.08,state.nearest.z);interactionRing.material.opacity=.36+Math.sin(now*.004)*.1;}}
  }else{
    if(riddenCamel)riddenCamel.visible=false;
    camera.position.set(-27+Math.sin(landingTime*.05)*.55,3.4+Math.sin(landingTime*.12)*.07,-23+Math.cos(landingTime*.05)*.3);camera.lookAt(-42,2,-44);
  }
  const remoteTime=motionClock.time(now,REMOTE_MOTION_DELAY_MS);
  for(const object of state.players.values()){
    const pose=object.userData.motion?.sample(remoteTime,object.userData.pose);if(!pose)continue;
    object.position.set(pose.x,pose.y,pose.z);object.rotation.y=pose.yaw;
    const station=state.snapshot?.cookiePractice?.stations?.find(s=>s.occupiedBy===object.userData.player?.id);animatePracticeAvatar(object.userData.practiceRig,!!station?.active,now/1000);
  }
  for(const object of state.animals.values()){
    const pose=object.userData.motion?.sample(remoteTime,object.userData.pose);if(!pose)continue;
    object.position.set(pose.x,pose.y+Math.sin(now*.0017+pose.x)*.002,pose.z);object.rotation.y=pose.yaw;
    const animal=object.userData.animal;
    if(animal.egg){if(object.userData.label)object.userData.label.visible=!!me&&Math.hypot(animal.x-me.x,animal.z-me.z)<16;continue;}
    const plastic=object.getObjectByName('rescue-plastic');if(plastic)plastic.visible=animal.need==='trapped';const wound=object.getObjectByName('animal-wound');if(wound)wound.visible=animal.need==='wounded'; const animalStatus=animal.need+'|'+animal.disposition; if(object.userData.lastNeed!==animalStatus){
      object.userData.lastNeed=animalStatus;const old=object.userData.label;if(old){old.parent?.remove(old);old.material.map.dispose();old.material.dispose();}
      const label=textSprite(pretty(animal.species)+(animal.need?' · '+pretty(animal.need):animal.disposition==='hostile'?' · ANGRY':''),'#fff3dc','#294b43b0',512,90);label.position.y=(animal.species==='whale'?3:1.4);label.scale.set(2.8,.5,1);object.add(label);object.userData.label=label;
    }
    if(object.userData.label)object.userData.label.visible=!!me&&Math.hypot(animal.x-me.x,animal.z-me.z)<22;
  }
  ambientAudio.update(state.snapshot,{enabled:state.connected&&state.hasPlayed&&!!state.player,hidden:document.hidden,listener:me?{...me,x:camera.position.x,y:camera.position.y,z:camera.position.z,yaw:state.yaw}:null,layout:state.layout,players:[...state.players.values()].map(o=>({...o.userData.player,x:o.position.x,y:o.position.y,z:o.position.z})),animals:[...state.animals.values()].filter(o=>o.userData.animal?.species==='dolphin').map(o=>({...o.userData.animal,x:o.position.x,y:o.position.y,z:o.position.z}))});
  const prediction=Math.min(.15,(now-state.lastSnapshot)/1000);
  for(const object of state.projectiles.values()){
    object.position.copy(object.userData.target).addScaledVector(object.userData.velocity,prediction);object.rotation.x+=dt*10;object.rotation.z+=dt*6;
  }
  for(const object of state.nodes.values()){
    if(object.userData.node.kind==='camel'){const pose=object.userData.motion?.sample(remoteTime,object.userData.pose);if(pose){object.position.set(pose.x,pose.y,pose.z);object.rotation.y=pose.yaw;}animateCamelView(object,{time:now/1000,moving:object.userData.node.phase==='following',eating:object.userData.node.phase==='eating'});}
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
import('./panels.js').then(({createPanels})=>{panels=createPanels({$,state,action,toast,chooseCookie,itemCount,itemName,cookieName,cookieNote,isKnown,cookieIcon,itemIcon,materialEmoji:{...materialEmoji,...bakingIngredientIcons},bakingItemName,cookieNames,cookieNotes,escapeHTML,pretty,beginBuild,cancelBuild,toggleBakeryBuild,insideOwnBakery:()=>insideOwnBakery(state.snapshot),resume:setPlaying});}).catch((error)=>{console.warn('Field guide unavailable',error);toast('The field guide could not load. Please reload the page.','error');});
api('/api/world').then(({layout})=>{if(layout&&scene&&state.layout?.version!==layout.version)createEnvironment(layout);}).catch(()=>{});
api('/api/session').then((session)=>{if(session.authenticated&&session.player)enterGame(session.player);}).catch(()=>{$('server-status').textContent='CONNECTING TO THE WORLD';});
if(matchMedia('(pointer: coarse)').matches){$('mobile-note').hidden=false;}
