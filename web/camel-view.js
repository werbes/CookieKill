// Shared camel rig and held treats, built with the game's existing low-poly palette.
export function heldCamelTreat(snapshot){
  const slot=snapshot?.me?.hotbar?.[snapshot?.me?.selectedSlot||0];
  return slot?.count>0?snapshot?.town?.treatCatalog?.find(t=>t.id===slot.item)||null:null;
}

export function heldTreatTarget(snapshot){
  const me=snapshot?.me;
  if(!me||me.ridingCamel||me.island==='camel_race'||!heldCamelTreat(snapshot))return null;
  return (snapshot.nodes||[]).filter(n=>n.kind==='camel'&&n.owner===me.id&&n.available!==false&&!!n.inCave===!!me.inCave&&Math.abs((n.y||0)-(me.y||0))<=2)
    .map(n=>({...n,distance:Math.hypot(n.x-me.x,n.z-me.z),isAnimal:false})).filter(n=>n.distance<=5).sort((a,b)=>a.distance-b.distance)[0]||null;
}

export function createCamelFoodView({THREE,sphere,box,cylinder},item){
  const root=new THREE.Group();root.name='camel-food-'+item;
  if(item==='happy_apple'){
    sphere(.16,'#c95e4f',-.055,0,0,root,1).scale.set(.8,.95,.9);
    sphere(.16,'#de7854',.055,0,0,root,1).scale.set(.8,.95,.9);
    cylinder(.014,.018,.13,'#7f6040',0,.19,0,root,5);
    const leaf=sphere(.085,'#73975e',.05,.2,0,root,1);leaf.scale.set(1,.2,.45);
  }else if(item==='spring_carrot'){
    const carrot=cylinder(.075,.012,.45,'#e99b44',0,0,0,root,7);carrot.rotation.z=-.2;
    for(let i=0;i<3;i++){const leaf=box(.04,.2,.02,'#6c9357',(i-1)*.045,.29,0,root);leaf.rotation.z=(i-1)*-.4;}
  }else if(item==='swift_dates'){
    box(.32,.055,.25,'#b9a17a',0,-.095,0,root);
    for(let i=0;i<4;i++)sphere(.07,i%2?'#8c543b':'#a96843',(i%2-.5)*.14,-.025,Math.floor(i/2)*.12-.06,root,1).scale.set(.65,.7,1.2);
  }else if(item==='stamina_oats'){
    cylinder(.15,.11,.28,'#c6ae7d',0,0,0,root,7);
    cylinder(.14,.14,.025,'#e6d39b',0,.15,0,root,7);
    for(let i=0;i<7;i++)sphere(.025,'#f1dca5',Math.sin(i*2.4)*.09,.17,Math.cos(i*2.4)*.09,root,0).scale.set(.55,1,.65);
  }
  root.rotation.z=.15;return root;
}

export function createCamelView({THREE,sphere,box,cylinder}){
  const root=new THREE.Group();root.name='camel-model';
  const body=new THREE.Group();body.name='camel-body';root.add(body);
  sphere(.85,'#caa370',0,1.5,0,body,1).scale.set(.6,.8,1.3);
  sphere(.5,'#b78e5c',0,2.12,.15,body);
  const legs=[];
  for(const x of [-.35,.35])for(const z of [-.65,.65]){
    const leg=new THREE.Group();leg.name='camel-leg-'+legs.length;leg.position.set(x,1.3,z);body.add(leg);
    cylinder(.1,.12,1.2,'#c09a66',0,-.6,0,leg);
    box(.22,.12,.3,'#97774e',0,-1.24,-.025,leg);legs.push(leg);
  }
  const neck=cylinder(.18,.28,1.5,'#cba573',0,2,-.95,body);neck.rotation.x=-.4;
  const head=new THREE.Group();head.name='camel-head';head.position.set(0,2.7,-1.35);body.add(head);
  sphere(.33,'#d3ad7b',0,0,0,head).scale.set(.7,.75,1.4);
  for(const x of [-.18,.18]){
    sphere(.037,'#354237',x,.06,-.15,head,1);
    const ear=sphere(.12,'#b78e5c',x,.22,.13,head,1);ear.scale.set(.6,1.4,.45);ear.rotation.z=-x*2;
  }
  const jaw=box(.28,.09,.28,'#c19a68',0,-.14,-.28,head);jaw.name='camel-jaw';
  box(.9,.12,.75,'#997374',0,2.27,.35,body);
  root.userData.camelRig={body,legs,head,jaw};return root;
}

export function animateCamelView(view,{time=0,moving=false,eating=false}={}){
  const model=view?.userData.camelRig?view:view?.getObjectByName('camel-model'),rig=model?.userData.camelRig;
  if(!rig)return;
  const stride=time*10;
  rig.legs.forEach((leg,i)=>{leg.rotation.x=moving?Math.sin(stride+(i===0||i===3?0:Math.PI))*.5:0;});
  rig.body.position.y=moving?Math.abs(Math.sin(stride))*.055:0;
  rig.head.rotation.x=eating?.16+Math.sin(time*9)*.08:moving?Math.sin(stride)*.025:0;
  rig.head.rotation.z=eating?Math.sin(time*7)*.035:0;
  rig.jaw.position.y=-.14+(eating?Math.sin(time*17)*.025:0);
}
