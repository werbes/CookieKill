import {clubReducedFlashing} from './club-settings.js';
import {CLUB_MULTIPLIERS,CLUB_CHANCES,CLUB_COLORS,CLUB_STARTS,clubSectorCenter} from './club-odds.js';

export const CLUB_X=-202,CLUB_FLOOR=-12,CLUB_STEPS=48,CLUB_RUN=20/CLUB_STEPS;
export const CLUB_OPENING={minX:-205,maxX:-199,minZ:-217,maxZ:-207};
export function forestClubFootprint(x,z){
  const half=z> -207?3+(-207-z)*(-1.4/3):3;
  return z<=-204&&z>=-227&&Math.abs(x-CLUB_X)<=half||x>=-218&&x<=-186&&z>=-250&&z<=-226;
}
export function forestClubGround(x,z){
  if(z<=-204&&z>=-227&&Math.abs(x-CLUB_X)<=(z> -207?3+(z+207)*1.4/3:3))return -Math.min(CLUB_STEPS,Math.ceil(Math.max(0,(-207-z)/CLUB_RUN-1e-8)))*.25||0;
  return x>=-218&&x<=-186&&z>=-250&&z<=-226?CLUB_FLOOR:0;
}

// Clip only the hidden stairwell aperture, preserving interpolated terrain
// colors. Accept both indexed terrain and the already-clipped desert terrain.
export function cutClubOpening(geometry,THREE){
  const p=geometry.attributes.position,c=geometry.attributes.color,index=geometry.index,vertices=[],colors=[];
  const {minX,maxX,minZ,maxZ}=CLUB_OPENING,planes=[v=>minX-v[0],v=>v[0]-maxX,v=>v[2]-maxZ,v=>minZ-v[2]];
  const clip=(polygon,plane,positive)=>{const out=[];for(let i=0;i<polygon.length;i++){const a=polygon[i],b=polygon[(i+1)%polygon.length],da=plane(a),db=plane(b),ain=positive?da>=0:da<=0,bin=positive?db>=0:db<=0;if(ain)out.push(a);if(ain!==bin){const t=da/(da-db);out.push(a.map((value,j)=>value+(b[j]-value)*t));}}return out;};
  const emit=polygon=>{for(let i=1;i<polygon.length-1;i++)for(const v of [polygon[0],polygon[i],polygon[i+1]]){vertices.push(...v.slice(0,3));if(c)colors.push(...v.slice(3));}};
  for(let i=0;i<(index?.count??p.count);i+=3){let polygon=[0,1,2].map(j=>{const n=index?index.getX(i+j):i+j;return [p.getX(n),p.getY(n),p.getZ(n),...(c?[c.getX(n),c.getY(n),c.getZ(n)]:[])];});if(polygon.every(v=>v[0]<=minX)||polygon.every(v=>v[0]>=maxX)||polygon.every(v=>v[2]<=minZ)||polygon.every(v=>v[2]>=maxZ)){emit(polygon);continue;}for(const plane of planes){emit(clip(polygon,plane,true));polygon=clip(polygon,plane,false);if(!polygon.length)break;}}
  geometry.setIndex(null);geometry.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));if(c)geometry.setAttribute('color',new THREE.Float32BufferAttribute(colors,3));geometry.deleteAttribute('normal');geometry.computeVertexNormals();
}

export function buildForestClub({THREE,parent,flatLabel,makePerson}){
  const group=new THREE.Group();group.name='forest-club';parent.add(group);
  const batches=new Map(),pose=new THREE.Object3D(),color=new THREE.Color(),lights=[];
  function instance(shape,shade,x,y,z,sx,sy,sz,ry=0,glow=false){const key=shape+(glow?'-glow':'');if(!batches.has(key))batches.set(key,[]);batches.get(key).push({shade,x,y,z,sx,sy,sz,ry});}
  const box=(w,h,d,shade,x,y,z,ry=0,glow=false)=>instance('box',shade,x,y,z,w,h,d,ry,glow);
  const ball=(r,shade,x,y,z,sx=1,sy=1,sz=1)=>instance('ball',shade,x,y,z,r*sx,r*sy,r*sz);
  const post=(r,h,shade,x,y,z)=>instance('post',shade,x,y,z,r,h,r);
  const label=(text,shade,x,y,z,w,h,ry=0)=>{const o=flatLabel(text,shade,null,768,128);o.position.set(x,y,z);o.scale.set(w,h,1);o.rotation.y=ry;group.add(o);return o;};
  const glow=(shade,x,y,z,intensity,range)=>{const l=new THREE.PointLight(shade,intensity,range,2);l.position.set(x,y,z);l.userData.clubIntensity=intensity;l.intensity=0;l.visible=false;group.add(l);lights.push(l);return l;};
  function surface(vertices,shade,name){const geometry=new THREE.BufferGeometry();geometry.setAttribute('position',new THREE.Float32BufferAttribute(vertices,3));geometry.computeVertexNormals();const o=new THREE.Mesh(geometry,new THREE.MeshStandardMaterial({color:shade,roughness:1,side:THREE.DoubleSide}));o.name=name;group.add(o);return o;}
  const quad=(a,b,c,d)=>[...a,...b,...c,...a,...c,...d];
  // A moss-covered rock arch shelters the genuine underground staircase.
  const sides=16,outer=(i,z)=>{const a=i*Math.PI/sides;return [CLUB_X-8.4*Math.cos(a),5.1*Math.sin(a)**.65,z];},inner=i=>{const a=i*Math.PI/sides;return [CLUB_X-4.4*Math.cos(a),3.75*Math.sin(a)**.65,-203.8];};
  const shell=[],face=[];
  for(let row=0;row<8;row++)for(let i=0;i<sides;i++)shell.push(...quad(outer(i,-203.6-row*1.8),outer(i,-205.4-row*1.8),outer(i+1,-205.4-row*1.8),outer(i+1,-203.6-row*1.8)));
  for(let i=0;i<sides;i++)face.push(...quad(inner(i),inner(i+1),outer(i+1,-203.6),outer(i,-203.6)));
  surface(shell,'#53625a','club-mossy-outcrop');surface(face,'#697665','club-entrance-arch');
  for(let i=0;i<36;i++){const side=i%2?-1:1,x=CLUB_X+side*(4.8+(i%5)*.66),z=-203.8-Math.floor(i/10)*1.1;ball(.42+(i%3)*.14,['#687b55','#4b6a4f','#84926b'][i%3],x,.4+Math.floor(i/5)%4,z,1.5,.38,.6);}
  for(const side of [-1,1]){box(.09,2.8,.09,'#72e2cd',CLUB_X+side*3.1,1.45,-203.63,0,true);post(.1,1.8,'#424b41',CLUB_X+side*4.7,.9,-201.9);ball(.18,'#c59f60',CLUB_X+side*4.7,1.86,-201.9);}
  label('THE ROOT ROOM','#b9f6ce',CLUB_X,3.55,-203.48,6.8,.82);label('DOWNSTAIRS / GOOD COMPANY','#d9bf96',CLUB_X,2.95,-203.46,4.9,.32);
  box(8.8,.12,3.2,'#59675b',CLUB_X,-.065,-205.45);
  for(let i=0;i<CLUB_STEPS;i++){const y=-(i+1)*.25,z=-207-(i+.5)*CLUB_RUN;box(6,.25,CLUB_RUN+.004,'#495752',CLUB_X,y-.125,z);box(5.85,.012,.04,i%4===0?'#c6b680':'#849083',CLUB_X,y+.009,-207-i*CLUB_RUN-.04);if(i%6===0)for(const side of [-1,1])box(.065,.1,.22,'#81cdb4',CLUB_X+side*2.96,y+.3,z,0,true);}
  const tunnel=[],at=(i,z)=>{const a=i*Math.PI/sides,t=Math.min(1,Math.max(0,(-204-z)/3)),floor=z> -207?0:Math.max(-12,(z+207)*.6);return [CLUB_X-(4.4-1.35*t)*Math.cos(a),floor+(3.75+.65*t)*Math.sin(a)**.6,z];};
  for(let row=0;row<48;row++){const z=-204-row*23/48,next=z-23/48;for(let i=0;i<sides;i++)tunnel.push(...quad(at(i,z),at(i+1,z),at(i+1,next),at(i,next)));}
  surface(tunnel,'#3b4e49','club-stair-tunnel');
  for(let i=0;i<4;i++)glow(i%2?'#e5b78d':'#8cdec9',CLUB_X,1.9-i*3,-206-i*5,11,9);
  // The room remains spacious; the bar and wheel occupy the perimeter.
  box(32,.22,24,'#252b34',-202,-12.12,-238);box(32,.25,24,'#19232d',-202,-6.38,-238);
  box(.4,5.5,24,'#2c3b3e',-218,-9.25,-238);box(.4,5.5,24,'#303343',-186,-9.25,-238);box(32,5.5,.4,'#303943',-202,-9.25,-250);
  box(13,5.5,.4,'#303b40',-211.5,-9.25,-226);box(13,5.5,.4,'#303b40',-192.5,-9.25,-226);
  for(const y of [-11.72,-7.05]){box(31.4,.04,.07,'#bc74d8',-202,y,-249.72,0,true);box(.07,.04,23.4,'#61cfc5',-217.72,y,-238,0,true);box(.07,.04,23.4,'#d39762',-186.28,y,-238,0,true);}
  label('THE ROOT ROOM','#f1bde0',-203,-7.5,-249.65,10.5,1);label('STAY A LITTLE LONGER','#a1cbd4',-203,-8.25,-249.63,6.2,.38);
  for(let i=0;i<9;i++){box(.4,1+(i%4)*.38,.08,['#576276','#94799c','#5b8c89'][i%3],-213+i*2.3,-10.15,-249.65);}
  // Eight chairs around four cocktail tables; coordinates match club_seat nodes.
  for(let row=0;row<2;row++)for(let i=0;i<4;i++){const x=-213+i*4,z=-245-row*3,back=row===0?.42:-.42;box(.98,.2,.92,row%2?'#745b85':'#376c69',x,-11.5,z);box(.98,.75,.17,row%2?'#745b85':'#376c69',x,-11.02,z+back);for(const side of [-1,1])for(const end of [-1,1])post(.055,.45,'#b39463',x+side*.36,-11.78,z+end*.32);}
  for(let i=0;i<4;i++){const x=-213+i*4,z=-246.5;post(.55,.12,'#9b7956',x,-11.12,z);post(.075,.8,'#71674f',x,-11.55,z);post(.32,.06,'#71674f',x,-11.97,z);post(.075,.2,'#debc86',x+.13,-10.96,z);ball(.085,'#c08f79',x+.13,-10.82,z);}
  // Warm wood, brass trim and a backlit wall of bottles balance the neon.
  box(2,1.15,8,'#4b514d',-215.5,-11.425,-232);box(2.15,.13,8.15,'#ab8059',-215.5,-10.79,-232);box(.05,.05,7.9,'#e6b872',-214.47,-11.62,-232,0,true);
  for(const y of [-10.7,-9.75,-8.8]){box(.7,.09,7.5,'#a17c55',-217.2,y,-232);box(.03,.05,7.35,'#cde5b7',-216.83,y-.06,-232,0,true);}
  for(let row=0;row<3;row++)for(let i=0;i<12;i++){const x=-217.12,z=-235.3+i*.6,y=-10.45+row*.95;post(.085,.36,['#639c89','#c19762','#7c86b8','#cb7d8a'][i%4],x,y,z);post(.035,.16,'#c7cdb2',x,y+.25,z);box(.01,.12,.13,'#e8dab0',x+.09,y,z);}
  label('NIGHT CAPS & MILKSHAKES','#f1d5ab',-216.75,-7.92,-232,5.8,.48,Math.PI/2);
  if(makePerson){const bartender=makePerson('','#405c62');bartender.position.set(-217,-12,-231);bartender.rotation.y=Math.PI/2;group.add(bartender);}
  glow('#e7ba84',-214.9,-8.8,-232,74,15);
  // Disco Duke brings his purple tailoring and rainbow cake stand out at night.
  // Keep everything in one group so dawn removes the vendor and his wares.
  const cakeSeller=new THREE.Group();cakeSeller.name='club-cake-seller';cakeSeller.position.set(-191,CLUB_FLOOR,-245);cakeSeller.rotation.y=2.25;cakeSeller.visible=false;group.add(cakeSeller);
  const sellerMaterials=new Map(),sellerGeometry={box:new THREE.BoxGeometry(1,1,1),post:new THREE.CylinderGeometry(1,1,1,12),ball:new THREE.IcosahedronGeometry(1,1)};
  function sellerPart(shape,shade,x,y,z,sx,sy,sz,name=''){
    if(!sellerMaterials.has(shade))sellerMaterials.set(shade,new THREE.MeshStandardMaterial({color:shade,roughness:.55,metalness:shade==='#ecc46a'?.5:0}));
    const o=new THREE.Mesh(sellerGeometry[shape],sellerMaterials.get(shade));o.position.set(x,y,z);o.scale.set(sx,sy,sz);o.name=name;cakeSeller.add(o);return o;
  }
  if(makePerson)cakeSeller.add(makePerson('','#9954bd',false,{pantsColor:'black'}));
  else{sellerPart('ball','#deb889',0,1.68,0,.25,.25,.25);for(const side of [-1,1])sellerPart('box','#352c42',side*.16,.36,0,.18,.65,.2);}
  const coat=sellerPart('post','#8f43ad',0,.93,0,.43,1.05,.32,'club-cake-seller-coat');
  coat.geometry=new THREE.CylinderGeometry(.7,1,1,8);
  for(const side of [-1,1]){
    sellerPart('post','#9b50ba',side*.36,1.13,0,.14,.6,.14);
    const lapel=sellerPart('box','#e8c797',side*.17,1.27,-.29,.12,.49,.045);lapel.rotation.z=-side*.28;
    sellerPart('box','#ecc46a',side*.17,.11,-.09,.24,.11,.38);
    sellerPart('box','#211d39',side*.125,1.71,-.235,.2,.105,.05);
  }
  sellerPart('box','#ecc46a',0,1.71,-.265,.06,.035,.025);
  sellerPart('post','#773a91',0,1.94,0,.64,.09,.52,'club-cake-seller-hat');
  sellerPart('post','#9b50ba',0,2.08,0,.31,.26,.28);
  sellerPart('post','#ecc46a',0,2,0,.32,.065,.29);
  const feather=sellerPart('ball','#59ddd2',.4,2.24,.025,.09,.38,.045,'club-cake-seller-feather');feather.rotation.z=-.35;
  sellerPart('ball','#f4b2d7',.49,2.47,.025,.075,.16,.04);
  const chain=new THREE.Mesh(new THREE.TorusGeometry(.19,.025,6,16,Math.PI),sellerMaterials.get('#ecc46a'));chain.name='club-cake-seller-chain';chain.position.set(0,1.36,-.325);chain.rotation.z=Math.PI;cakeSeller.add(chain);
  sellerPart('ball','#ecc46a',0,1.11,-.33,.065,.09,.03);
  sellerPart('post','#ecc46a',-.66,.6,-.13,.036,1.13,.036,'club-cake-seller-cane');
  sellerPart('ball','#ecc46a',-.66,1.19,-.13,.105,.09,.075);
  sellerPart('post','#352c42',-.66,.06,-.13,.052,.09,.052);
  // A low brass stand keeps the cakes visible without hiding the outfit.
  sellerPart('post','#ecc46a',0,.45,-1.13,.065,.84,.065);
  sellerPart('post','#554361',0,.055,-1.13,.55,.1,.45);
  sellerPart('box','#ecc46a',0,.88,-1.13,1.8,.09,.9,'club-funny-cake-tray');
  sellerPart('box','#efe0ba',0,.937,-1.13,1.68,.03,.79);
  for(let i=0;i<5;i++){
    const x=-.64+i*.32,z=-1.13+(i%2?-.15:.14);
    sellerPart('post','#c18a54',x,1.04,z,.125,.18,.125);
    sellerPart('post',['#c589e3','#6ed9c0','#e6bd70','#e8a3be','#88bce6'][i],x,1.155,z,.14,.065,.14,'club-funny-cake');
    sellerPart('ball','#fff1cf',x,1.22,z,.035,.04,.035);
  }
  const sellerSign=flatLabel('DISCO DUKE / FUNNY CAKES','#f8d791','#392442',768,128);sellerSign.name='club-cake-seller-sign';sellerSign.position.set(0,2.94,-.1);sellerSign.rotation.y=Math.PI;sellerSign.scale.set(3.8,.63,1);cakeSeller.add(sellerSign);
  const sellerLight=new THREE.PointLight('#e8b8d7',15,5,2);sellerLight.position.set(-.8,2.8,-1.6);cakeSeller.add(sellerLight);
  // Each sector has the same proportional width as its server probability.
  box(2,1.2,2,'#514650',-188.5,-11.4,-237);box(2.12,.12,2.12,'#b79464',-188.5,-10.75,-237);
  label('FORTUNE WHEEL','#efc794',-186.5,-7.12,-237,5,.55,-Math.PI/2);
  const wheel=new THREE.Group();wheel.name='club-fortune-wheel';wheel.position.set(-187.25,-8.85,-237);wheel.rotation.y=-Math.PI/2;group.add(wheel);
  const sectors=CLUB_MULTIPLIERS,sectorColors=CLUB_COLORS;
  for(let i=0;i<7;i++){const wedge=new THREE.Mesh(new THREE.CircleGeometry(1.35,12,CLUB_STARTS[i]*Math.PI*2/100,CLUB_CHANCES[i]*Math.PI*2/100),new THREE.MeshBasicMaterial({color:sectorColors[i],side:THREE.DoubleSide}));wheel.add(wedge);const a=clubSectorCenter(i)*Math.PI*2,t=flatLabel(sectors[i]+'x','#fff0c7',null,128,80);t.scale.set(.55,.32,1);t.position.set(Math.cos(a)*.89,Math.sin(a)*.89,.02);wheel.add(t);}
  const rim=new THREE.Mesh(new THREE.TorusGeometry(1.39,.07,6,56),new THREE.MeshStandardMaterial({color:'#cdaa73',metalness:.55,roughness:.35}));wheel.add(rim);
  const hub=new THREE.Mesh(new THREE.CircleGeometry(.23,24),new THREE.MeshBasicMaterial({color:'#ecd9a4',side:THREE.DoubleSide}));hub.position.z=.04;wheel.add(hub);
  box(.35,.35,.35,'#e5bd78',-187.29,-7.2,-237,Math.PI/4,true);glow('#d9a2cd',-190,-8.5,-237,46,13);
  // One instanced mesh for the entire disco floor; smooth color drift, no cuts.
  const floor=new THREE.InstancedMesh(new THREE.BoxGeometry(1.16,.045,1.16),new THREE.MeshBasicMaterial({color:0xffffff}),80);floor.name='club-disco-floor';
  for(let i=0;i<80;i++){pose.position.set(-208.625+(i%10)*1.25,-11.96,-240.375+Math.floor(i/10)*1.25);pose.rotation.set(0,0,0);pose.scale.set(1,1,1);pose.updateMatrix();floor.setMatrixAt(i,pose.matrix);color.setHSL((i*.137)%1,.45,.36);floor.setColorAt(i,color);}floor.computeBoundingSphere();group.add(floor);
  const mirror=new THREE.Mesh(new THREE.IcosahedronGeometry(.72,2),new THREE.MeshStandardMaterial({color:'#c2d7d9',metalness:.86,roughness:.19,flatShading:true}));mirror.name='club-mirror-ball';mirror.position.set(-202,-7.6,-236);group.add(mirror);post(.035,.85,'#949ca2',-202,-6.95,-236);
  const mirrorTiles=new THREE.InstancedMesh(new THREE.PlaneGeometry(.11,.11),new THREE.MeshBasicMaterial({color:'#e0e1d1',side:THREE.DoubleSide}),128);
  for(let i=0;i<128;i++){const u=(i+.5)/128,y=1-2*u,a=i*2.399963;pose.position.set(Math.sqrt(1-y*y)*Math.cos(a)*.735,y*.735,Math.sqrt(1-y*y)*Math.sin(a)*.735);pose.lookAt(0,0,0);pose.scale.set(1,1,1);pose.updateMatrix();mirrorTiles.setMatrixAt(i,pose.matrix);}mirror.add(mirrorTiles);
  const fixtures=[];
  for(let i=0;i<4;i++){const x=-209+(i%2)*14,z=-229-Math.floor(i/2)*12;box(.55,.28,.55,'#17212b',x,-6.9,z);const disc=new THREE.Mesh(new THREE.CircleGeometry(.19,12),new THREE.MeshBasicMaterial({color:i%2?'#e9b2d4':'#a5ddd7'}));disc.position.set(x,-7.05,z);disc.rotation.x=-Math.PI/2;group.add(disc);fixtures.push(disc);}
  // Steady pools of light reveal the upholstery, tabletops and woodwork;
  // only the small ceiling lenses pulse, never these room lights.
  glow('#9398d9',-203,-8.3,-236,46,21);
  glow('#d69cbd',-212,-8.8,-246,64,14);
  glow('#a1c6d8',-201,-8.8,-246,64,15);
  // The many steps, bottles, chairs and trim become six draw calls at most.
  for(const [key,items] of batches){const shape=key.split('-')[0],geometry=shape==='box'?new THREE.BoxGeometry(1,1,1):shape==='ball'?new THREE.IcosahedronGeometry(1,1):new THREE.CylinderGeometry(1,1,1,10),material=key.endsWith('glow')?new THREE.MeshBasicMaterial({color:0xffffff}):new THREE.MeshStandardMaterial({color:0xffffff,roughness:.78});const batch=new THREE.InstancedMesh(geometry,material,items.length);batch.name='club-static-'+key;items.forEach((p,i)=>{pose.position.set(p.x,p.y,p.z);pose.rotation.set(0,p.ry,0);pose.scale.set(p.sx,p.sy,p.sz);pose.updateMatrix();batch.setMatrixAt(i,pose.matrix);batch.setColorAt(i,color.set(p.shade));});batch.computeBoundingSphere();batch.receiveShadow=true;group.add(batch);}
  let lastReduced=null,lastColorTime=-Infinity,lastResult=0,wheelFrom=0,wheelTo=0,wheelStart=0;
  function update(time,active=true,result=null,night=false){
    cakeSeller.visible=!!night;sellerLight.visible=!!active;
    const reduced=clubReducedFlashing();for(const l of lights){l.visible=active;l.intensity=active?l.userData.clubIntensity:0;}
    if(!active)return;
    if(lastReduced!==reduced||!reduced&&time-lastColorTime>.1){for(let i=0;i<80;i++){color.setHSL((i*.137+(reduced?0:time*.025))%1,.45,.36);floor.setColorAt(i,color);}floor.instanceColor.needsUpdate=true;lastColorTime=time;lastReduced=reduced;}
    mirror.rotation.y=reduced?0:time*.17;
    // Four tiny ceiling lenses breathe at 1.5Hz. Ambient light never flashes.
    for(let i=0;i<fixtures.length;i++)fixtures[i].material.color.setScalar(reduced?.65:.64+.12*Math.sin(time*Math.PI*3+i));
    if(result?.id&&result.id!==lastResult){lastResult=result.id;wheelFrom=wheel.rotation.z;const target=Math.PI/2-clubSectorCenter(result.sector)*Math.PI*2;wheelTo=target+Math.ceil((wheelFrom-target)/ (Math.PI*2))*Math.PI*2+(reduced?0:Math.PI*4);wheelStart=time;}
    if(lastResult){const t=reduced?1:Math.min(1,(time-wheelStart)/2.6);wheel.rotation.z=wheelFrom+(wheelTo-wheelFrom)*(1-(1-t)**3);}
  }
  return {group,lights,update};
}
