import {activityArt,activityLabel,activityCookie,disposeActivity} from './cookie-activity-art.js';

export const SOUTH_TREE={x:210,z:184,deckY:12};
export const COOKIE_RING_COVER=[{x:-8,z:-5,width:4,depth:3,height:2.2},{x:8,z:5,width:4,depth:3,height:2.2},{x:-8,z:8,width:3,depth:5,height:1.4},{x:8,z:-8,width:3,depth:5,height:1.4}];

export function createTreeRingScene({THREE,scene,worldParent,flatLabel}){
  const tree=new THREE.Group();tree.name='south-cookie-tree';worldParent.add(tree);const t=activityArt(THREE,tree);
  const trunk=t.post(1.5,12,'#896442',210,6,184,tree,true);trunk.name='cookie-tree-trunk';
  // Two open turns of thin wooden treads wind around the plain trunk.
  // Their tops, radial widths and rotations match the physical stairs exactly.
  const stairRecords=[],startAngle=13*Math.PI/16,angleStep=2*Math.PI/32;
  const at=(angle,radius,y)=>[210+Math.cos(angle)*radius,y,184+Math.sin(angle)*radius];
  for(let i=0;i<64;i++){
    const angle=startAngle+(i+.5)*angleStep,top=(i+1)*.1875,[x,,z]=at(angle,3.4,top);
    const step=t.box(2.2,.16,1.05,i%2?'#bb955f':'#b08a58',x,top-.08,z);step.rotation.y=-angle;
    stairRecords.push({x,z,top,width:2.2,depth:1.05,rotation:-angle});
    const a=startAngle+i*angleStep,b=a+angleStep;
    t.beam(at(a,4.5,i*.1875+.95),at(b,4.5,top+.95),.035,'#d8ba83');
    if(i%4===0){const [px,,pz]=at(a,4.5,top);t.post(.055,.95,'#8d6a41',px,i*.1875+.475,pz);t.beam(at(angle,1.48,Math.max(.08,top-.5)),at(angle,3.3,top-.11),.09,'#8b6946');}
  }
  const [railX,,railZ]=at(startAngle+64*angleStep,4.5,12);t.post(.055,.95,'#8d6a41',railX,12.475,railZ);
  tree.userData.steps=stairRecords;
  t.box(14,.3,10,'#be9b66',210,11.85,179);
  t.box(2.8,.3,4,'#be9b66',207.6,11.85,184.8);
  for(const x of [206.22,208.98]){t.beam([x,12.9,186.78],[x,12.9,184.05],.035,'#d8ba83');for(const z of [184.1,186.7])t.post(.055,.95,'#8d6a41',x,12.475,z);}
  // High supporting limbs sit beneath the unchanged, full-size hut floor.
  for(const [x,z]of [[203.5,174.7],[216.5,174.7],[204,183],[216,183]])t.beam([210,11.72,183.5],[x,11.84,z],.2,'#826240');
  for(const [x,z,w,d]of [[210,173.7,14,.6],[217.3,179,.6,10],[202.7,179,.6,10],[213.5,184.3,7,.6]]){
    t.box(w,1.1,d,'#b28a55',x,12.55,z);t.box(w,.6,d,'#a57d4e',x,15.5,z);
    t.box(w,2.1,d*.9,'#96aea2',x,14.15,z);
    t.box(w,.07,d+.02,'#e0c995',x,14.05,z);
  }
  for(const [x,z]of [[203,174],[217,174],[217,184],[210,184],[203,177]])t.post(.16,4,'#8b6541',x,14,z);
  t.box(14,.3,10,'#735b43',210,16.15,179);
  for(let i=0;i<10;i++)t.box(14.9,.075,.8,i%2?'#917146':'#9c7b4e',210,16.2,174.5+i);
  // Branches above the roof hold a broad canopy without enclosing the stairs.
  for(const [x,z,dx,dz]of [[204,176,-5,-4],[216,181,6,4],[205,183,-5,5],[214,176,5,-8]]){
    t.beam([x,16.15,z],[x+dx,20,z+dz],.45,'#826240');
    const crown=t.ball(5.8,'#607b4b',x+dx,21,z+dz);crown.scale.y*=.55;
  }
  for(let i=0;i<13;i++){const a=i*2.4,r=i%3?7:2,crown=t.ball(5.1+i%3*.5,['#69864e','#7b9458','#526e43'][i%3],210+Math.sin(a)*r,22+(i%3)*1.6,183+Math.cos(a)*r);crown.scale.y*=.55;}
  for(const angle of [0,2.1,4.2])t.beam(at(angle,1.1,.55),at(angle,2,.06),.2,'#957448');
  t.post(.06,1.7,'#8d6a41',205.1,.85,189.5);
  activityLabel(tree,flatLabel,'THE COOKIE TREE',205.1,1.9,189.5,3.6,.55);
  activityLabel(tree,flatLabel,'FOLLOW THE SPIRAL STAIRS',205.1,1.4,189.51,3.6,.35,0,'#f7e5bd','#806544');
  activityLabel(tree,flatLabel,'GOLDEN COOKIE / FIGHTING RING',213,14.75,177.5,4,.55,-Math.PI/2,'#ffebad','#806441');
  activityLabel(tree,flatLabel,'SILVER COOKIE / PRACTICE & GYM',215,14.75,181.5,4,.55,-Math.PI/2,'#f1f4e9','#5d7377');
  const gold=activityCookie(THREE,t,tree,'tree-golden-cookie','#edc663',213,13.3,177.5),silver=activityCookie(THREE,t,tree,'tree-silver-cookie','#c8d6d5',215,13.3,181.5);t.batch();
  const ring=new THREE.Group();ring.name='cookie-fighting-ring';ring.visible=false;scene.add(ring);const r=activityArt(THREE,ring);
  r.box(90,.12,80,'#a1ac79',0,-.13,0);r.box(50,.12,42,'#cbb78a',0,-.06,0);r.box(46,.025,38,'#d9c79a',0,.015,0);
  for(const side of [-1,1]){
    r.box(1,1.4,42,'#a48a61',side*24.5,.7,0);r.box(50,1.4,1,'#a48a61',0,.7,side*20.5);
    // Five-metre netting explains the server's solid arena boundary while
    // preserving the view of the spectator benches outside it.
    for(const north of [false,true]){const wire=new THREE.LineSegments(new THREE.WireframeGeometry(new THREE.PlaneGeometry(north?50:42,3.6,north?25:21,6)),new THREE.LineBasicMaterial({color:'#7c8169',transparent:true,opacity:.6}));wire.position.set(north?0:side*24.5,3.2,north?side*20.5:0);if(!north)wire.rotation.y=Math.PI/2;ring.add(wire);}
    for(let row=0;row<3;row++){
      r.box(46,.4,1.2,row%2?'#907055':'#a2845d',0,.3+row*.6,side*(23+row*1.7));
      r.box(1.2,.4,40,'#9f815a',side*(27+row*1.7),.3+row*.6,0);
    }
    for(let x=-24;x<=24;x+=6){r.post(.12,5,'#806f53',x,2.5,side*20.5);r.box(2.1,.7,.055,(x/6)%2?'#b98675':'#71998f',x,2.6,side*20);}
  }
  COOKIE_RING_COVER.forEach((cover,i)=>{r.box(cover.width,cover.height,cover.depth,i<2?'#b59365':'#9caa78',cover.x,cover.height/2,cover.z);r.box(cover.width,.1,cover.depth,'#d3bd8d',cover.x,cover.height-.05,cover.z);});
  ring.userData.cover=COOKIE_RING_COVER;
  for(const [i,[x,z]]of [[-20,-16],[20,16],[-20,16],[20,-16]].entries()){
    r.post(1.1,.025,i%2?'#9eb9a4':'#d6a18c',x,.023,z);
    const number=activityLabel(ring,flatLabel,String(i+1),x,.042,z,.8,.8);number.rotation.x=-Math.PI/2;
  }
  activityLabel(ring,flatLabel,'COOKIE CLASH',0,5.2,-21,11,1.4);
  activityLabel(ring,flatLabel,'W A S D MOVE / LEFT CLICK THROW / R DETAILS & RETURN',0,3.7,-21,13,.6);
  const exit=activityCookie(THREE,r,ring,'cookie-ring-exit','#edc663',0,1.4,17);activityLabel(ring,flatLabel,'E / RETURN TO THE TREE HUT',0,2.9,17,5,.55);r.batch();
  function sync(snapshot){ring.visible=snapshot?.me?.island==='cookie_ring';}
  function update(time){for(const [portal,base]of [[gold,13.3],[silver,13.3],[exit,1.4]]){portal.position.y=base+Math.sin(time*1.7)*.1;portal.rotation.y=time*.35;}}
  return {tree,ring,sync,update,dispose(){disposeActivity(tree);disposeActivity(ring);}};
}
