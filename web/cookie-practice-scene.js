import {activityArt,activityLabel,activityCookie,disposeActivity} from './cookie-activity-art.js';

export const PRACTICE_STATIONS=[{id:'practice_treadmill',kind:'treadmill',x:-16,z:6,width:2.4,depth:4},{id:'practice_bench_press',kind:'bench_press',x:-8,z:6,width:2.2,depth:3},{id:'practice_leg_press',kind:'leg_press',x:-16,z:14,width:2.2,depth:3}];
export const PRACTICE_POOL={minX:8,maxX:20,minZ:-8,maxZ:16,lanes:[10,14,18]};

export function createCookiePracticeScene({THREE,scene,flatLabel}){
  const group=new THREE.Group();group.name='cookie-practice-gym';group.visible=false;scene.add(group);const a=activityArt(THREE,group),targets=new Map(),rigs=new Map();
  // Four surrounding slabs leave the physical pool depression open.
  for(const [x,z,w,d]of [[-8.5,0,33,50],[22.5,0,5,50],[14,-16.5,12,17],[14,20.5,12,9]])a.box(w,.16,d,'#d2c8a7',x,-.08,z);
  a.box(68,.2,65,'#8fa57a',0,-.82,0);
  for(const side of [-1,1]){a.box(.14,1.2,50,'#a5916b',side*25,.6,0);a.box(50,1.2,.14,'#a5916b',0,.6,side*25);}
  a.box(45,.02,.14,'#e9e7ce',0,.024,-11);
  activityLabel(group,flatLabel,'COOKIE AIM SCHOOL',-3,4.6,-24,12,1.15);
  activityLabel(group,flatLabel,'UNLIMITED SUGAR COOKIES / LEFT CLICK TO THROW',-3,3.6,-24,13,.6);
  for(let i=0;i<4;i++){
    const root=new THREE.Group();root.name='practice-target-'+i;root.position.set(i===3?12:-16+i*8,1.8,-21);group.add(root);
    // Target rings face the shooting line at +Z; all motion comes from snapshots.
    const disks=[];for(const [radius,depth,color]of [[.75,.12,'#eee7c9'],[.52,.13,'#b76e63'],[.29,.14,'#ead8ac'],[.105,.15,'#b66e61']]){const disk=a.post(radius,depth,color,0,0,.02,root,true);disk.rotation.x=Math.PI/2;disk.material=disk.material.clone();disks.push(disk);}
    a.post(.075,1.1,'#987951',0,-1.2,0,root,true);a.box(1,.08,.75,'#957750',0,-1.72,0,root,true);
    const label=activityLabel(root,flatLabel,i===3?'MOVING':'TARGET '+(i+1),0,1.06,0,1.9,.35);label.name='practice-target-label';targets.set('practice_target_'+i,{root,disks});
  }
  activityLabel(group,flatLabel,'THE PHYSICAL GYM',-24.7,3.2,10,9,.8,Math.PI/2);
  activityLabel(group,flatLabel,'E USE / W RUN / SPACE REP / Q STOP',-24.7,2.35,10,10,.5,Math.PI/2);
  for(const station of PRACTICE_STATIONS){
    const root=new THREE.Group();root.name=station.id;root.position.set(station.x,0,station.z);group.add(root);rigs.set(station.id,{root,kind:station.kind});
    a.box(station.width,.04,station.depth,'#808e88',0,.025,0,root);
    if(station.kind==='treadmill'){
      a.box(2.2,.2,3.8,'#536962',0,.12,0,root);a.box(1.6,.025,3.2,'#394e49',0,.24,0,root);
      const stripes=[];for(let i=0;i<8;i++)stripes.push(a.box(1.55,.012,.09,'#86988a',0,.26,-1.4+i*.4,root,true));rigs.get(station.id).stripes=stripes;
      for(const x of [-1,1]){a.post(.07,1.35,'#b2bab0',x,.8,-1.2,root);a.beam([x,1.45,-1.2],[x,1.1,.7],.055,'#b2bab0',root);}
      const panel=a.box(1.5,.36,.16,'#687e73',0,1.38,-1.43,root);panel.rotation.x=-.3;a.box(.67,.15,.04,'#bcd5bc',0,1.44,-1.31,root);
    }else if(station.kind==='bench_press'){
      a.box(.65,.2,2.2,'#b27969',0,.45,.1,root);for(const z of [-.8,.9])a.box(.7,.45,.14,'#697d76',0,.225,z,root);
      for(const x of [-.85,.85]){a.post(.07,1.5,'#8a9b91',x,.75,-.8,root);a.box(.32,.08,.45,'#93a296',x,1.2,-.8,root);}
      const bar=new THREE.Group();bar.name='practice-bench-bar';bar.position.set(0,1.38,-.7);root.add(bar);const shaft=a.post(.035,2,'#c6c8b6',0,0,0,bar,true);shaft.rotation.z=Math.PI/2;
      for(const x of [-.85,.85]){const weight=a.post(.25,.17,'#51625b',x,0,0,bar,true);weight.rotation.z=Math.PI/2;}rigs.get(station.id).bar=bar;
    }else{
      a.box(.75,.2,.85,'#a98671',0,.38,.55,root);const back=a.box(.8,.8,.15,'#ad8971',0,.73,.98,root);back.rotation.x=-.25;
      for(const x of [-.78,.78])a.beam([x,.2,.5],[x,1.25,-1.15],.07,'#8a9b91',root);
      const press=new THREE.Group();press.name='practice-leg-plate';press.position.set(0,.88,-.65);root.add(press);const plate=a.box(1.4,.65,.12,'#647c70',0,0,0,press,true);plate.rotation.x=-.4;rigs.get(station.id).press=press;
    }
    const title=activityLabel(root,flatLabel,station.kind==='treadmill'?'TREADMILL / STAMINA':station.kind==='bench_press'?'BENCH PRESS / ARMS':'LEG PRESS / LEGS',0,2.5,-1.65,3.9,.45);title.name=station.id+'-label';rigs.get(station.id).label=title;
  }
  // The pool water lies at zero; swimmers and the bottom are .6m below it.
  a.box(12,.12,24,'#7aa8ac',14,-.67,4);
  const water=new THREE.Mesh(new THREE.PlaneGeometry(12,24),new THREE.MeshStandardMaterial({color:'#70b7bf',transparent:true,opacity:.72,roughness:.28,metalness:.08,side:THREE.DoubleSide}));water.name='practice-pool-water';water.rotation.x=-Math.PI/2;water.position.set(14,.003,4);group.add(water);
  for(const x of [8,20])a.box(.24,.18,24.5,'#e9e4c9',x,.03,4);for(const z of [-8,16])a.box(12.4,.18,.24,'#e9e4c9',14,.03,z);
  for(const x of [12,16])for(let i=0;i<24;i++)a.ball(.075,i%2?'#e6dbb5':'#ba8276',x,.055,-7.5+i);
  for(const x of PRACTICE_POOL.lanes){a.box(.08,.01,23,'#4d8391',x,-.603,4);for(const z of [-5,13])a.box(2.5,.012,.12,'#d9dfa9',x,.012,z);}
  activityLabel(group,flatLabel,'SWIM LANES / 13 → −5 → 13',6.8,2.7,4,10,.6,Math.PI/2,'#eff4df','#628f94');
  activityLabel(group,flatLabel,'SWIM ONE FULL RETURN LAP / 2 COINS PER LEVEL',6.8,1.85,4,10,.45,Math.PI/2,'#eaf4e6','#628f94');
  const gate=a.box(2.4,.035,.55,'#f8dc7c',10,.08,13,group,true);gate.name='practice-swim-next-gate';gate.visible=false;
  const exit=activityCookie(THREE,a,group,'cookie-practice-exit','#c5d4d3',0,1.35,22);activityLabel(group,flatLabel,'E / RETURN TO THE TREE HUT',0,2.85,22,5,.6);a.batch();
  let snapshot=null,lastAnimation=0,syncedAt=0;
  function sync(value){snapshot=value;syncedAt=lastAnimation;group.visible=value?.me?.island==='cookie_practice';if(!group.visible)return;const view=value.cookiePractice;for(const target of view?.targets||[]){const rig=targets.get(target.id);if(rig)rig.root.position.set(target.x,target.y,target.z);}for(const [id,rig]of rigs)rig.label.visible=view?.exercise?.station!==id;const next=view?.exercise?.station==='practice_pool'?view.exercise.nextGate:null;gate.visible=!!next;if(next)gate.position.set(next.x,.08,next.z);}
  function update(time){
    lastAnimation=time;if(!group.visible)return;const view=snapshot?.cookiePractice||{},worldTime=(snapshot?.time||0)+Math.min(.2,Math.max(0,time-syncedAt));
    for(const target of view.targets||[]){const rig=targets.get(target.id);if(!rig)continue;if(target.moving)rig.root.position.x=12+Math.sin(worldTime*.8)*5;const flash=target.flashUntil>worldTime;for(const disk of rig.disks){disk.material.emissive.set(flash?'#c19b39':'#000000');disk.material.emissiveIntensity=flash?.28:0;}}
    for(const station of view.stations||[]){const rig=rigs.get(station.id);if(!rig)continue;const active=!!station.active;
      if(rig.stripes)rig.stripes.forEach((stripe,i)=>stripe.position.z=-1.4+((i*.4+(active?time*1.7:0))%3.2));
      if(rig.bar)rig.bar.position.y=1.38+(active?Math.sin(time*6)*.19:0);
      if(rig.press){rig.press.position.z=-.65-(active?(Math.sin(time*6)+1)*.15:0);rig.press.position.y=.88+(active?(Math.sin(time*6)+1)*.09:0);}
    }
    exit.position.y=1.35+Math.sin(time*1.7)*.1;exit.rotation.y=time*.35;water.material.opacity=.7+Math.sin(time*.8)*.025;
  }
  return {group,sync,update,dispose(){disposeActivity(group);}};
}

export function paintCookieActivityMap(canvas,snapshot,mini=false){
  const ctx=canvas?.getContext('2d');if(!ctx)return;const ring=snapshot?.me?.island==='cookie_ring',w=canvas.width,h=canvas.height,scale=Math.min(w,h)/(mini?58:62),px=x=>w/2+x*scale,pz=z=>h/2+z*scale;
  ctx.clearRect(0,0,w,h);ctx.fillStyle='#78946f';ctx.fillRect(0,0,w,h);ctx.fillStyle='#d5c397';ctx.fillRect(px(ring?-24:-25),pz(ring?-20:-25),(ring?48:50)*scale,(ring?40:50)*scale);
  if(ring){for(const [x,z,bw,bd]of [[-8,-5,4,3],[8,5,4,3],[-8,8,3,5],[8,-8,3,5]]){ctx.fillStyle='#9d855e';ctx.fillRect(px(x-bw/2),pz(z-bd/2),bw*scale,bd*scale);}}
  else{
    ctx.fillStyle='#6cb4bc';ctx.fillRect(px(8),pz(-8),12*scale,24*scale);ctx.strokeStyle='#e7d7a8';for(const x of [12,16]){ctx.beginPath();ctx.moveTo(px(x),pz(-8));ctx.lineTo(px(x),pz(16));ctx.stroke();}
    for(const s of snapshot.cookiePractice?.stations||PRACTICE_STATIONS){if(s.id==='practice_pool')continue;ctx.fillStyle='#6c8475';ctx.fillRect(px(s.x-(s.width||2)/2),pz(s.z-(s.depth||3)/2),(s.width||2)*scale,(s.depth||3)*scale);}
    for(const t of snapshot.cookiePractice?.targets||[]){ctx.beginPath();ctx.fillStyle='#b97969';ctx.arc(px(t.x),pz(t.z),mini?2:4,0,Math.PI*2);ctx.fill();}
    const gate=snapshot.cookiePractice?.exercise?.station==='practice_pool'?snapshot.cookiePractice.exercise.nextGate:null;if(gate){ctx.fillStyle='#ffe288';ctx.fillRect(px(gate.x-1),pz(gate.z-.4),2*scale,.8*scale);}
  }
  for(const p of snapshot?.players||[]){ctx.fillStyle='#f6f0d9';ctx.beginPath();ctx.arc(px(p.x),pz(p.z),mini?2:4,0,Math.PI*2);ctx.fill();}
  const p=snapshot?.me;if(p){ctx.save();ctx.translate(px(p.x),pz(p.z));ctx.rotate(-(p.yaw||0));ctx.fillStyle='#ffc765';ctx.beginPath();ctx.moveTo(0,-(mini?5:8));ctx.lineTo(mini?3:5,mini?4:6);ctx.lineTo(mini?-3:-5,mini?4:6);ctx.closePath();ctx.fill();ctx.restore();}
  ctx.fillStyle='#ffe9af';ctx.beginPath();ctx.arc(px(0),pz(ring?17:22),mini?3:5,0,Math.PI*2);ctx.fill();if(!mini){ctx.fillStyle='#354d42';ctx.font='bold 15px Arial';ctx.textAlign='center';ctx.fillText(ring?'COOKIE FIGHTING RING':'AIM PRACTICE & PHYSICAL GYM',w/2,22);}
}
