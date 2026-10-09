import {createMotionTrack} from './motion.js';

const colors={brown:'#947353',cream:'#e3cfa9',pink:'#d69e98',dark:'#403c37'};
const speciesStyle={
  wood_mouse:{size:.74,fur:'#9c7958',belly:'#e2cda8'},house_mouse:{size:.72,fur:'#929490',belly:'#d9d8ca'},
  rat:{size:1.1,fur:'#6a6863',belly:'#aaa796'},blind_albino_rat:{size:1.1,fur:'#eee9df',belly:'#fff8ed',eye:'#c54c57'},
  cactus_mouse:{size:.73,fur:'#cdad74',belly:'#f4e2b9',ears:1.3},
  squirrel:{size:1.18,fur:'#b97944',belly:'#edd1a3',bushy:true},flying_squirrel:{size:1.12,fur:'#8d7964',belly:'#e8d9ba',bushy:true,glider:true},
  chipmunk:{size:.9,fur:'#b88953',belly:'#e4c697',striped:true},mole_rat:{size:.94,fur:'#cfac9c',belly:'#e4c6b3',mole:true},
  beaver:{size:1.45,fur:'#866041',belly:'#b18a61',beaver:true},hamster:{size:.98,fur:'#d5a057',belly:'#fff0c7',round:true},
  otter:{size:1.5,fur:'#8b6b4e',belly:'#d7c4a0',otter:true}
};

// Static body parts are merged into one colored mesh; only paws, tails and the
// glider membranes remain separate so a busy woodland stays inexpensive.
export function createRodentModel(THREE,species){
  const cfg=speciesStyle[species]||speciesStyle.wood_mouse,root=new THREE.Group(),body=new THREE.Group();
  root.name='rodent-model-'+species;root.scale.setScalar(cfg.size);root.add(body);
  const material=new THREE.MeshStandardMaterial({vertexColors:true,roughness:.95,flatShading:true}),parts=[],names=[];
  function part(name,geometry,color,x,y,z,sx=1,sy=1,sz=1,rotation=0){
    const m=new THREE.Matrix4().compose(new THREE.Vector3(x,y,z),new THREE.Quaternion().setFromEuler(new THREE.Euler(0,0,rotation)),new THREE.Vector3(sx,sy,sz));
    const g=geometry.index?geometry.toNonIndexed():geometry;g.applyMatrix4(m);if(g!==geometry)geometry.dispose();
    const c=new THREE.Color(color),values=new Float32Array(g.attributes.position.count*3);for(let i=0;i<values.length;i+=3){values[i]=c.r;values[i+1]=c.g;values[i+2]=c.b;}g.setAttribute('color',new THREE.BufferAttribute(values,3));parts.push(g);names.push(name);
  }
  const ball=(name,color,x,y,z,sx,sy,sz)=>part(name,new THREE.IcosahedronGeometry(1,1),color,x,y,z,sx,sy,sz);
  const box=(name,color,x,y,z,w,h,d)=>part(name,new THREE.BoxGeometry(w,h,d),color,x,y,z);
  const length=cfg.otter?.62:cfg.round?.33:.43,width=cfg.beaver?.29:cfg.round?.27:.21;
  ball('body',cfg.fur,0,.3,.03,width,cfg.round?.28:.23,length);
  ball('belly',cfg.belly,0,.22,-.04,width*.89,.15,length*.9);
  const headZ=cfg.otter?-.51:cfg.round?-.27:-.37;
  ball('head',cfg.fur,0,.38,headZ,cfg.beaver?.23:.18,.19,.21);
  ball('muzzle',cfg.belly,0,.3,headZ-.15,cfg.mole?.14:.12,.09,cfg.beaver?.12:.17);
  ball('nose',cfg.mole?colors.pink:'#514640',0,.325,headZ-.29,.045,.033,.037);
  for(const side of [-1,1]){
    const ear=cfg.otter||cfg.beaver||cfg.mole?.065:.12*(cfg.ears||1);
    ball('ear',cfg.fur,side*.14,.53,headZ+.02,ear,ear,.047);
    ball('inner-ear',colors.pink,side*.145,.54,headZ-.02,ear*.64,ear*.68,.013);
    ball('eye',cfg.eye||'#292e2d',side*.14,.405,headZ-.135,cfg.mole?.012:.027,.027,.024);
    if(!cfg.mole)ball('eye-glint','#fff9e9',side*.147,.415,headZ-.153,.006,.006,.006);
    if(cfg.round)ball('cheek',cfg.belly,side*.13,.29,headZ-.1,.105,.11,.11);
    if(cfg.beaver||cfg.mole)box('front-tooth','#f4ddb0',side*.043,.238,headZ-.233,.054,.105,.045);
    if(cfg.striped){box('cream-stripe','#eddfb5',side*.087,.505,.025,.034,.018,.51);box('dark-stripe','#594632',side*.132,.48,.025,.026,.018,.5);}
    if(species==='squirrel')part('ear-tuft',new THREE.ConeGeometry(.035,.12,4),cfg.fur,side*.14,.68,headZ+.02);
  }
  if(cfg.mole)for(let i=0;i<5;i++)box('skin-fold','#b18c82',0,.505,.2-i*.075,.31,.017,.012);
  const merged=new THREE.BufferGeometry();
  for(const attribute of ['position','normal','color']){const values=parts.flatMap(g=>Array.from(g.attributes[attribute].array));merged.setAttribute(attribute,new THREE.Float32BufferAttribute(values,3));}
  parts.forEach(g=>g.dispose());merged.computeBoundingSphere();const skin=new THREE.Mesh(merged,material);skin.castShadow=true;body.add(skin);
  const limbMaterial=new THREE.MeshStandardMaterial({color:cfg.fur,roughness:1,flatShading:true}),limbs=[];
  for(const side of [-1,1])for(const front of [-1,1]){
    const leg=new THREE.Group();leg.position.set(side*width*.77,.19,front*(length*.66));body.add(leg);
    const foot=new THREE.Mesh(new THREE.BoxGeometry(cfg.beaver?.14:.085,.1,.19),limbMaterial);foot.position.set(0,-.115,-.03);foot.castShadow=true;leg.add(foot);limbs.push(leg);
  }
  const tail=new THREE.Group();tail.position.set(0,.26,length*.88);body.add(tail);
  if(cfg.beaver){const paddle=new THREE.Mesh(new THREE.SphereGeometry(1,8,5),limbMaterial);paddle.scale.set(.18,.055,.39);paddle.position.set(0,-.1,.27);tail.add(paddle);}
  else if(cfg.bushy){for(let i=0;i<3;i++){const plume=new THREE.Mesh(new THREE.IcosahedronGeometry(1,1),limbMaterial);plume.scale.set(.17+i*.035,.23,.17+i*.022);plume.position.set(0,.06+i*.2,.14-Math.max(0,i-1)*.07);tail.add(plume);}}
  else if(!cfg.round){for(let i=0;i<3;i++){const segment=new THREE.Mesh(new THREE.CylinderGeometry((cfg.otter?.075:.02)*(1-i*.25),(cfg.otter?.075:.02)*(1-i*.25),.23,6),limbMaterial);segment.rotation.x=Math.PI/2;segment.position.set(Math.sin(i*.7)*.055,-.08,.1+i*.18);tail.add(segment);}}
  const wings=[];
  if(cfg.glider)for(const side of [-1,1]){
    const membrane=new THREE.BufferGeometry();membrane.setAttribute('position',new THREE.Float32BufferAttribute([0,0,-.32,side*.4,-.03,-.24,side*.36,-.05,.32,0,0,-.32,side*.36,-.05,.32,0,0,.32],3));membrane.computeVertexNormals();
    const wing=new THREE.Mesh(membrane,new THREE.MeshStandardMaterial({color:'#c6ae89',side:THREE.DoubleSide,roughness:1}));wing.position.set(side*.1,.29,0);body.add(wing);wings.push(wing);
  }
  const halo=new THREE.Mesh(new THREE.TorusGeometry(.48,.025,5,24),new THREE.MeshBasicMaterial({color:'#ffd2ef',transparent:true,opacity:.7}));halo.position.y=.66;halo.rotation.x=Math.PI/2;halo.visible=false;root.add(halo);
  root.userData.rodentRig={body,limbs,tail,wings,halo,size:cfg.size};root.userData.species=species;root.userData.parts=names;
  animateRodentModel(root,0,{state:'wandering',moving:false});return root;
}

export function animateRodentModel(model,time,{state='wandering',moving=false}={}){
  const rig=model.userData.rodentRig;if(!rig)return;
  const stride=moving?Math.sin(time*15)*.35:0;
  rig.limbs.forEach((limb,i)=>limb.rotation.x=stride*(i%2?-1:1));
  rig.body.position.y=moving?Math.abs(Math.sin(time*15))*.015:Math.sin(time*2)*.004;
  rig.body.scale.y=state==='burrowing'?.35:1;rig.body.rotation.x=state==='diving'?.4:state==='rainbow'?-.35:0;
  rig.tail.rotation.y=Math.sin(time*(moving?9:2))*(moving?.13:.045);
  rig.wings.forEach((wing,i)=>wing.rotation.z=state==='gliding'?0:(i?1:-1)*1.1);
  rig.halo.visible=state==='rainbow';if(rig.halo.visible){rig.halo.rotation.z=time*.4;rig.halo.material.color.setHSL((time*.045)%1,.6,.75);}
}

function disposeModel(group){const geometries=new Set(),materials=new Set();group.traverse(o=>{if(o.geometry)geometries.add(o.geometry);if(o.material)for(const m of [].concat(o.material))materials.add(m);});geometries.forEach(g=>g.dispose());materials.forEach(m=>m.dispose());}

export function createRodentsScene({THREE,scene}){
  const group=new THREE.Group();group.name='small-wildlife';scene.add(group);const animals=new Map();let world='';
  function sync(snapshot){
    const me=snapshot.me||{},nextWorld=(me.island||'')+'|'+!!me.inCave;if(world!==nextWorld){for(const entry of animals.values()){group.remove(entry.model);disposeModel(entry.model);}animals.clear();world=nextWorld;}
    const views=[...(snapshot.rodents||[]),...(snapshot.nutIsland?.pets||[]).map(p=>({...p,island:me.island,state:'wandering',pet:true}))],seen=new Set();
    for(const view of views){
      if((view.island||'')!==(me.island||'')||me.inCave)continue;seen.add(view.id);let entry=animals.get(view.id);
      if(entry&&entry.species!==view.species){group.remove(entry.model);disposeModel(entry.model);animals.delete(view.id);entry=null;}
      if(!entry){const model=createRodentModel(THREE,view.species);model.name='small-animal-'+view.id;model.userData.rodentID=view.id;group.add(model);entry={model,species:view.species,track:createMotionTrack({maxIntervalMs:1000,teleportDistance:12}),pose:{},last:null,moving:false};animals.set(view.id,entry);}
      entry.moving=entry.last?Math.hypot(view.x-entry.last.x,view.z-entry.last.z)>.004:false;entry.last={x:view.x,z:view.z};entry.state=view.state||'wandering';entry.model.visible=entry.state!=='hidden';
      entry.track.push((snapshot.time||0)*1000,{x:view.x,y:view.y||0,z:view.z,yaw:view.yaw||0},{key:world});
    }
    for(const [id,entry] of animals)if(!seen.has(id)){group.remove(entry.model);disposeModel(entry.model);animals.delete(id);}
  }
  function update(time){for(const entry of animals.values()){if(entry.track.sample(time,entry.pose)){entry.model.position.set(entry.pose.x,entry.pose.y,entry.pose.z);entry.model.rotation.y=entry.pose.yaw;}animateRodentModel(entry.model,time/1000,{state:entry.state,moving:entry.moving});}}
  function dispose(){for(const entry of animals.values())disposeModel(entry.model);animals.clear();scene.remove(group);}
  return {group,sync,update,dispose};
}

export function createRodentRainbowEffect({document}){
  const layer=document.createElement('div');layer.id='otter-rainbow';layer.setAttribute('aria-hidden','true');
  Object.assign(layer.style,{position:'fixed',inset:'0',pointerEvents:'none',zIndex:'3',overflow:'hidden',opacity:'0',display:'none'});
  const tint=document.createElement('div'),ribbons=document.createElement('div');
  Object.assign(tint.style,{position:'absolute',inset:'0',pointerEvents:'none',mixBlendMode:'color',background:'linear-gradient(125deg,#ff9fc4,#efcf78,#94d8a0,#8dcbe5,#cab3ed)',opacity:'.18'});
  // Colored bands stay in the upper sky. Normal blending keeps them visible at night;
  // slow drifting and soft fades never flash or move the player's view.
  const colors=['#ff728f','#ffac60','#ffe877','#91e597','#70dce8','#8aa8ff','#c294ef'];
  const bands=colors.map((color,i)=>`<path d="M -100 ${400+i*10} C 180 ${-25+i*10} 615 ${-40+i*10} 970 ${200+i*10}"/><path d="M 1550 ${360+i*8} C 1330 ${90+i*8} 1150 ${62+i*8} 1040 ${170+i*8}"/>`.replaceAll('<path ',`<path stroke="${color}" `)).join('');
  const svg=`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1440 1000" preserveAspectRatio="none"><g fill="none" stroke-width="11" stroke-linecap="round">${bands}</g></svg>`;
  Object.assign(ribbons.style,{position:'absolute',inset:'-2%',pointerEvents:'none',backgroundImage:'url("data:image/svg+xml,'+encodeURIComponent(svg)+'")',backgroundSize:'100% 100%',backgroundRepeat:'no-repeat',opacity:'.62',filter:'blur(.7px)'});
  layer.appendChild(tint);layer.appendChild(ribbons);document.body.appendChild(layer);
  function update({until=0,time=0,reduced=false,enabled=true}={}){
    const remaining=Math.max(0,until-time);
    if(!enabled||!remaining){layer.style.display='none';layer.style.opacity='0';return;}
    const fade=Math.max(0,Math.min(1,remaining/3,(60-remaining)/2));
    layer.style.display='block';layer.style.opacity=String(fade);
    tint.style.opacity=reduced?'.1':'.18';tint.style.filter=reduced?'none':'hue-rotate('+((time*8)%360)+'deg)';
    ribbons.style.opacity=reduced?'.36':'.62';ribbons.style.transform=reduced?'none':`translate(${Math.sin(time*.12)*1.2}vw,${Math.cos(time*.1)*.7}vh)`;
  }
  return {layer,tint,ribbons,update,dispose(){layer.remove();}};
}
