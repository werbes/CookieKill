// The beach entrance stays at surface level; only the race uses a separate world.
function parts(THREE,parent){
  const materials=new Map();
  const material=color=>{if(!materials.has(color))materials.set(color,new THREE.MeshStandardMaterial({color,roughness:.86,flatShading:true}));return materials.get(color);};
  const mesh=(geometry,color,x,y,z)=>{const o=new THREE.Mesh(geometry,material(color));o.position.set(x,y,z);o.receiveShadow=true;parent.add(o);return o;};
  return {mesh,box:(w,h,d,c,x,y,z)=>mesh(new THREE.BoxGeometry(w,h,d),c,x,y,z),ball:(r,c,x,y,z)=>mesh(new THREE.IcosahedronGeometry(r,1),c,x,y,z),post:(r,h,c,x,y,z)=>mesh(new THREE.CylinderGeometry(r,r,h,8),c,x,y,z)};
}

export function buildBeachRaceCave({THREE,parent,flatLabel}){
  const group=new THREE.Group();group.name='beach-race-cliff';parent.add(group);
  const {box,ball,post}=parts(THREE,group);
  box(23,.1,20,'#dbc99e',-227.5,-.06,18);
  // Match the server's solid bounds and its six-metre east-facing entrance.
  box(1,9,20,'#8a948c',-238.5,4.5,18);
  box(22,8,1,'#929b8f',-227,4,8.5);box(22,8,1,'#a0a28f',-227,4,27.5);
  box(1,8,10,'#989e90',-216.5,4,13);box(1,8,4,'#a4a694',-216.5,4,26);
  box(1,3.5,6,'#8e9687',-216.5,6.25,21);
  box(23,3.5,20,'#858f81',-227.5,6.25,18);
  for(let i=0;i<15;i++){
    const x=-238+(i%5)*4.9,z=9+Math.floor(i/5)*8.5;
    ball(2.1+(i%3)*.35,['#8d9885','#a5ab95','#788b70'][i%3],x,8.1+(i%2)*.7,z).scale.set(1.25,.65,1.1);
  }
  // Thin strata and grassy caps make this read as an exposed coastal cliff.
  for(const y of [1.2,2.7,4.25,6.4])for(const [z,d] of [[13,9.6],[26,3.5]])box(.05,.1,d,'#c0baa1',-215.96,y,z);
  for(const side of [-1,1]){post(.11,2.1,'#6c735f',-214.7,1.05,21+side*3.2);ball(.23,'#ffd991',-214.7,2.2,21+side*3.2);}
  const sign=flatLabel('DUNE DASH / CAMEL RACING','#ffe9b3','#364e4c',768,128);sign.position.set(-215.9,3.85,21);sign.rotation.y=Math.PI/2;sign.scale.set(5.7,.74,1);group.add(sign);
  const inside=new THREE.PointLight('#f6d7a2',55,18,2);inside.position.set(-225,3.4,20);group.add(inside);
  const candy=new THREE.Group();candy.name='camel-race-lollipop';candy.position.set(-227,0,20);group.add(candy);
  const p=parts(THREE,candy);
  p.post(.9,.16,'#b1b3a0',0,.08,0);p.post(.075,1.3,'#fff1da',0,.8,0);
  p.ball(.5,'#e9b366',0,1.8,0).scale.set(.65,.68,1.25);
  p.ball(.32,'#d79764',0,2.1,.03);p.post(.12,.75,'#edbf80',0,2.04,-.48).rotation.x=-.35;
  p.ball(.25,'#efc481',0,2.45,-.66).scale.set(.7,.75,1.35);
  for(const x of [-.2,.2])for(const z of [-.36,.36])p.post(.055,.48,'#e5ad65',x,1.36,z);
  p.box(.65,.08,.52,'#75c8bb',0,2.05,.18);p.ball(.04,'#493d44',.15,2.49,-.72);
  const label=flatLabel('EAT A CAMEL LOLLIPOP TO ENTER','#f9eac9','#405d57',768,128);label.position.set(-229,3.1,20);label.rotation.y=Math.PI/2;label.scale.set(5.5,.7,1);group.add(label);
  return {group};
}

export function createCamelRaceScene({THREE,scene,flatLabel}){
  const group=new THREE.Group();group.name='camel-racing-ring';group.visible=false;scene.add(group);
  const {box,ball,post}=parts(THREE,group);
  box(330,.12,280,'#c6b58b',0,-.13,0);
  const ring=new THREE.Shape();ring.absellipse(0,0,60,42,0,Math.PI*2,false,0);
  const hole=new THREE.Path();hole.absellipse(0,0,43,25,0,Math.PI*2,true,0);ring.holes.push(hole);
  const road=new THREE.Mesh(new THREE.ShapeGeometry(ring,96),new THREE.MeshStandardMaterial({color:'#ead3a4',roughness:1,side:THREE.DoubleSide}));road.rotation.x=-Math.PI/2;road.position.y=.012;road.name='camel-race-track';group.add(road);
  const infield=new THREE.Mesh(new THREE.CircleGeometry(1,96),new THREE.MeshStandardMaterial({color:'#91a16d'}));infield.rotation.x=-Math.PI/2;infield.scale.set(42.8,24.8,1);infield.position.y=.015;group.add(infield);
  const pose=new THREE.Object3D(),railItems=[];
  for(const [rx,rz] of [[60,42],[43,25]])for(let i=0;i<128;i++){
    const a=i*Math.PI*2/128,b=(i+1)*Math.PI*2/128,x=rx*Math.cos(a),z=rz*Math.sin(a),nx=rx*Math.cos(b),nz=rz*Math.sin(b);
    if(i%2===0)railItems.push([x,.8,z,.14,1.6,.14,0]);
    for(const y of [.55,1.1])railItems.push([(x+nx)/2,y,(z+nz)/2,Math.hypot(nx-x,nz-z)+.08,.09,.09,-Math.atan2(nz-z,nx-x)]);
  }
  const rails=new THREE.InstancedMesh(new THREE.BoxGeometry(1,1,1),new THREE.MeshStandardMaterial({color:'#f3e7c8',roughness:.9}),railItems.length);rails.name='camel-race-fences';
  railItems.forEach(([x,y,z,sx,sy,sz,ry],i)=>{pose.position.set(x,y,z);pose.rotation.set(0,ry,0);pose.scale.set(sx,sy,sz);pose.updateMatrix();rails.setMatrixAt(i,pose.matrix);});rails.computeBoundingSphere();group.add(rails);
  for(let i=0;i<16;i++){
    const a=i*Math.PI*2/16;
    for(const [rx,rz] of [[60,42],[43,25]]){
      const x=rx*Math.cos(a),z=rz*Math.sin(a);post(.11,3.4,'#ad8455',x,1.7,z);
      const flag=box(.08,.75,1.35,i%2?'#cb8273':'#62a99c',x,3.12,z);flag.rotation.y=-a;
    }
    const arrow=new THREE.Mesh(new THREE.ConeGeometry(.7,1.6,3),new THREE.MeshBasicMaterial({color:'#c4944d'}));arrow.name='camel-race-direction-'+i;arrow.position.set(51.5*Math.cos(a),.035,33.5*Math.sin(a));arrow.rotation.set(Math.PI/2,0,Math.atan2(51.5*Math.sin(a),33.5*Math.cos(a)));group.add(arrow);
  }
  for(let row=0;row<2;row++)for(let i=0;i<17;i++)box(1,.024,.7,(i+row)%2?'#fff2d2':'#474f4b',43.5+i,.025,row*.7-.7);
  for(const x of [42,61])box(.45,6,.45,'#49665f',x,3,-.1);
  box(19.4,.65,.55,'#49665f',51.5,6,-.1);
  const banner=flatLabel('DUNE DASH / FINISH','#f9e9bf','#49665f',768,128);banner.position.set(51.5,6,-.42);banner.rotation.y=Math.PI;banner.scale.set(14,.95,1);group.add(banner);
  // A small shaded grandstand, podium and oasis give the ring a clear identity.
  for(const sign of [-1,1])for(let row=0;row<3;row++)box(32,.55,2,'#b8996c',0,.28+row*.55,sign*(49+row*2));
  for(const z of [-49,49])for(const x of [-17,17])post(.22,6.5,'#896c4a',x,3.25,z);
  for(const z of [-53,53])box(37,.2,12,'#e2be83',0,6.6,z);
  const pool=new THREE.Mesh(new THREE.CircleGeometry(7,32),new THREE.MeshStandardMaterial({color:'#72b3b0',roughness:.3}));pool.rotation.x=-Math.PI/2;pool.position.set(-10,.05,0);pool.scale.set(1.7,1,1);group.add(pool);
  for(const [x,h,label] of [[0,1.4,'1'],[-3,.9,'2'],[3,.55,'3']]){box(2.7,h,2.4,'#cfaf71',x,h/2,8);const t=flatLabel(label,'#fff2c9','#7f6950',128,128);t.position.set(x,h/2,9.22);t.scale.set(.7,.7,1);group.add(t);}
  for(let i=0;i<7;i++){const a=i*2.4,x=-23+Math.cos(a)*8,z=Math.sin(a)*8;post(.22,5,'#aa8658',x,2.5,z);for(let j=0;j<5;j++){const leaf=ball(1.7,'#698e61',x+Math.cos(j*1.26),5.1,z+Math.sin(j*1.26));leaf.scale.set(1,.2,.5);leaf.rotation.y=-j*1.26;}}
  const gate=new THREE.Mesh(new THREE.BoxGeometry(15,.06,2),new THREE.MeshBasicMaterial({color:'#ffe38a',transparent:true,opacity:.5,depthWrite:false}));gate.name='camel-race-next-checkpoint';gate.position.y=.09;group.add(gate);
  let snapshot=null;
  function sync(next){snapshot=next;group.visible=next.me?.island==='camel_race';if(!group.visible)return;const race=next.camelRace,entrant=race?.entrants?.find(e=>e.id===next.me.id);gate.visible=race?.phase==='racing'&&entrant?.status==='racing';const n=entrant?.checkpoint??1,a=n*Math.PI*2/(race?.track?.checkpoints||16);gate.position.set(51.5*Math.cos(a),.09,33.5*Math.sin(a));gate.rotation.y=-a;}
  function update(){if(snapshot&&group.visible)sync(snapshot);}
  return {group,sync,update};
}

export function paintRaceMap(canvas,snapshot,mini=false){
  const ctx=canvas?.getContext('2d');if(!ctx)return;
  const w=canvas.width,h=canvas.height,scale=Math.min(w/146,h/112),px=x=>w/2+x*scale,pz=z=>h/2+z*scale;
  ctx.clearRect(0,0,w,h);ctx.fillStyle='#c5b389';ctx.fillRect(0,0,w,h);
  for(const [rx,rz,color] of [[60,42,'#e8d1a1'],[43,25,'#879d6b']]){ctx.beginPath();ctx.ellipse(w/2,h/2,rx*scale,rz*scale,0,0,Math.PI*2);ctx.fillStyle=color;ctx.fill();ctx.strokeStyle='#f7e7bf';ctx.lineWidth=2;ctx.stroke();}
  ctx.strokeStyle='#534e45';ctx.beginPath();ctx.moveTo(px(43),pz(0));ctx.lineTo(px(60),pz(0));ctx.stroke();
  for(const p of [...(snapshot.players||[]),snapshot.me].filter(Boolean)){ctx.fillStyle=p.id===snapshot.me.id?'#ffe38a':'#ad6570';ctx.beginPath();ctx.arc(px(p.x),pz(p.z),mini?3:5,0,Math.PI*2);ctx.fill();}
  if(!mini){ctx.fillStyle='#334c43';ctx.font='bold 18px Arial';ctx.textAlign='center';ctx.fillText('DUNE DASH · THREE LAPS',w/2,28);}
}
