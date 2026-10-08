// Only exercising player avatars are rigged here; shopkeepers keep their own rigs.
export function createPracticeAvatarRig(THREE,body,posture){
  const legs=[],arms=[];
  for(const child of body.children)if(child.isGroup&&Math.abs(child.position.x-.37)<.01&&Math.abs(child.position.y-.97)<.01)child.visible=false;
  for(const side of [-1,1]){
    const leg=new THREE.Group();leg.position.set(side*.16,.71,0);body.add(leg);legs.push(leg);
    const arm=new THREE.Group();arm.position.set(side*.35,1.37,0);body.add(arm);arms.push(arm);
    for(const child of [...body.children]){
      if(!child.isMesh)continue;let pivot=null;
      if(Math.abs(child.position.x-side*.16)<.025&&child.position.y<.76)pivot=leg;
      else if(Math.abs(child.position.x-side*.35)<.045&&child.position.y>.75&&child.position.y<1.45)pivot=arm;
      if(pivot){child.position.sub(pivot.position);pivot.add(child);}
    }
  }
  if(posture==='bench_press'){body.rotation.x=-Math.PI/2;body.position.set(0,.06,.8);}
  if(posture==='leg_press'){body.position.y=-.16;body.rotation.x=.14;}
  return {body,legs,arms,posture};
}

export function animatePracticeAvatar(rig,active,time){
  if(!rig)return;const {body,legs,arms,posture}=rig,cycle=active?Math.sin(time*(posture==='treadmill'?8:6)):0;
  if(posture==='treadmill'){
    legs.forEach((leg,i)=>leg.rotation.x=(i?1:-1)*cycle*.58);arms.forEach((arm,i)=>arm.rotation.x=(i?-1:1)*cycle*.38);body.position.y=active?Math.abs(cycle)*.03:0;
  }else if(posture==='bench_press'){
    for(const arm of arms)arm.rotation.x=-Math.PI/2+cycle*.25;
    for(const leg of legs)leg.rotation.x=.15;
  }else if(posture==='leg_press'){
    for(const leg of legs)leg.rotation.x=-1.12+cycle*.23;
    arms.forEach((arm,i)=>{arm.rotation.x=-.28;arm.rotation.z=i?.22:-.22;});
  }
}
