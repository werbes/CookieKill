function shapes(THREE,parent){
  const materials=new Map();
  const mesh=(geometry,color,x,y,z)=>{if(!materials.has(color))materials.set(color,new THREE.MeshStandardMaterial({color,roughness:.9,flatShading:true}));const o=new THREE.Mesh(geometry,materials.get(color));o.position.set(x,y,z);o.receiveShadow=true;parent.add(o);return o;};
  return {box:(w,h,d,c,x,y,z)=>mesh(new THREE.BoxGeometry(w,h,d),c,x,y,z),ball:(r,c,x,y,z)=>mesh(new THREE.IcosahedronGeometry(r,1),c,x,y,z),post:(r,h,c,x,y,z)=>mesh(new THREE.CylinderGeometry(r,r,h,7),c,x,y,z)};
}
export function buildCamelTrainingEntrance({THREE,parent,flatLabel}){
  const group=new THREE.Group();group.name='pink-camel-training-lollipop';group.position.set(-231,0,14);parent.add(group);
  const {box,ball,post}=shapes(THREE,group);
  post(.8,.14,'#adada0',0,.07,0);post(.065,1.3,'#fff0da',0,.8,0);
  ball(.48,'#e8a4ba',0,1.8,0).scale.set(.65,.68,1.25);ball(.3,'#d989ad',0,2.1,.03);
  post(.12,.75,'#f0b8c8',0,2.04,-.48).rotation.x=-.35;ball(.25,'#f5c8d6',0,2.45,-.66).scale.set(.7,.75,1.35);
  for(const x of [-.2,.2])for(const z of [-.36,.36])post(.05,.48,'#e8a0b6',x,1.36,z);
  box(.65,.08,.52,'#fcdf98',0,2.05,.18);ball(.035,'#574151',.15,2.49,-.72);
  const label=flatLabel('PINK CAMEL / TRAINING GARDENS','#fff0de','#79596f',768,128);label.position.set(-1,3.1,0);label.rotation.y=Math.PI/2;label.scale.set(5,.68,1);group.add(label);
  return {group};
}

export function createCamelTrainingScene({THREE,scene,flatLabel}){
  const group=new THREE.Group();group.name='camel-training-gardens';group.visible=false;scene.add(group);
  const {box,ball,post}=shapes(THREE,group),courses=new Map();
  box(150,.14,100,'#bfc58c',0,-.12,0);box(112,.045,64,'#d7c79c',0,-.015,0);
  const defs=[['speed',-42,'PACE TRACK','#78b5aa'],['jump',-14,'JUMP SCHOOL','#dba371'],['stamina',14,'ENDURANCE','#83a3c5'],['happiness',42,'CALM CORNERS','#d399b5']];
  for(const [id,cx,title,color]of defs){
    const course=new THREE.Group();course.name='training-course-'+id;group.add(course);courses.set(id,course);
    const p=shapes(THREE,course);
    for(const side of [-1,1]){p.box(5,.04,53,'#f0deb4',cx+side*8,.025,0);p.box(21,.042,5,'#f0deb4',cx,.026,side*24);}
    p.box(10.5,.055,42,'#a9bb81',cx,.018,0);
    for(const [i,[x,z]]of [[cx+8,24],[cx-8,24],[cx-8,-24],[cx+8,-24]].entries()){
      const pad=p.box(3,.035,3,color,x,.057,z);pad.name='training-pad-'+i;
      for(const dx of [-2,2]){p.post(.06,1.6,'#94744f',x+dx,.8,z);p.box(.65,.36,.035,color,x+dx,1.45,z);}
      const number=flatLabel(String(i+1),'#fff7df','#6a775e',128,128);number.position.set(x,.082,z);number.rotation.x=-Math.PI/2;number.scale.set(.85,.85,1);course.add(number);
    }
    // Tangent arrows follow the server's east-south, west-south, west-north, east-north order.
    for(const [x,z,angle]of [[cx+8,-10,Math.PI],[cx,24,Math.PI/2],[cx-8,10,0],[cx,-24,-Math.PI/2]]){
      const arrow=new THREE.Mesh(new THREE.ConeGeometry(.45,1.2,3),new THREE.MeshBasicMaterial({color}));arrow.position.set(x,.07,z);arrow.rotation.set(-Math.PI/2,0,angle);course.add(arrow);
    }
    for(const side of [-1,1]){
      const board=flatLabel(title,'#fff4d8',color,768,128);board.position.set(cx,3.1,side*.04);board.rotation.y=side<0?Math.PI:0;board.scale.set(11,1.55,1);course.add(board);
    }
    for(const x of [cx-4.6,cx+4.6])p.post(.12,3,'#8f7956',x,1.5,0);
    if(id==='jump')for(const z of [-12,0,12]){
      const hurdle=p.box(4.8,.13,.18,'#e8b3c6',cx+8,.5,z);hurdle.name='training-hurdle';
      for(const dx of [-2.4,2.4])p.post(.085,.58,'#916f52',cx+8+dx,.29,z);
    }
    if(id==='happiness')for(let i=0;i<14;i++){const x=cx+Math.sin(i*2.4)*3.5,z=Math.cos(i*2.4)*14;p.post(.025,.35,'#799355',x,.2,z);p.ball(.15,i%2?'#e2a4b8':'#f0d18c',x,.42,z).scale.y=.35;}
    else for(const z of [-12,12]){p.post(.13,3,'#aa8961',cx,1.5,z);p.ball(1.6,'#89a467',cx,3.2,z).scale.y=.38;}
  }
  // Perimeter posts leave the exercise lanes unobstructed.
  for(let x=-54;x<=54;x+=6)for(const z of [-32,32])post(.09,1.1,'#a59168',x,.55,z);
  const gate=box(3.3,.06,3.3,'#ffe78e',0,.1,0);gate.name='camel-training-next-gate';
  function sync(snapshot){
    group.visible=snapshot.me?.island==='camel_training';if(!group.visible)return;
    const t=snapshot.camelTraining,course=t?.courses?.find(c=>c.id===t.exercise),point=course?.checkpoints?.[t?.checkpoint||0];gate.visible=!!point;
    if(point)gate.position.set(point.x,.1,point.z);
  }
  function update(now){if(group.visible){gate.material.emissive.set('#c2a23b');gate.material.emissiveIntensity=.2+Math.sin(now*3)*.13;}}
  return {group,sync,update,dispose(){group.removeFromParent();group.traverse(o=>{o.geometry?.dispose();});}};
}

export function paintTrainingMap(canvas,snapshot,mini=false){
  const ctx=canvas?.getContext('2d');if(!ctx)return;const w=canvas.width,h=canvas.height,scale=Math.min(w/125,h/80),px=x=>w/2+x*scale,pz=z=>h/2+z*scale;
  ctx.clearRect(0,0,w,h);ctx.fillStyle='#b4c28b';ctx.fillRect(0,0,w,h);
  const t=snapshot.camelTraining;ctx.lineWidth=Math.max(2,5*scale);
  for(const c of t?.courses||[]){ctx.strokeStyle=c.id===t.exercise?'#ffe09a':'#e8d9b1';ctx.strokeRect(px(c.centerX-c.halfWidth),pz(-c.halfDepth),c.halfWidth*2*scale,c.halfDepth*2*scale);if(!mini){ctx.fillStyle='#485c44';ctx.textAlign='center';ctx.font='11px Arial';ctx.fillText(c.name,px(c.centerX),pz(0));}}
  const point=t?.courses?.find(c=>c.id===t.exercise)?.checkpoints?.[t?.checkpoint||0];if(point){ctx.fillStyle='#be7294';ctx.beginPath();ctx.arc(px(point.x),pz(point.z),mini?3:6,0,Math.PI*2);ctx.fill();}
  for(const p of [...(snapshot.players||[]),snapshot.me].filter(Boolean)){ctx.fillStyle=p.id===snapshot.me.id?'#d49539':'#fff6df';ctx.beginPath();ctx.arc(px(p.x),pz(p.z),mini?3:5,0,Math.PI*2);ctx.fill();}
}
