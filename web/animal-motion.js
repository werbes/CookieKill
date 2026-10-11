// Shared with internal/game/animal_motion.go. Route times are world seconds;
// the descriptor stays unchanged until an animal reacts or its state changes.
export function sampleAnimalMotion(animal,time,out={}) {
  const motion=animal.motion;
  if(!motion||!['orbit','still'].includes(motion.kind)||!Number.isFinite(time))return null;
  const elapsed=Math.max(0,time-motion.epoch);
  let x=motion.fromX,z=motion.fromZ,heading=motion.heading,y=.02,speed=0;
  if(motion.kind==='orbit'){
    const theta=motion.phase+motion.angularSpeed*elapsed;
    const targetX=motion.centerX+Math.sin(theta)*motion.radius+.3*Math.sin(time*.67+motion.wanderPhase);
    const targetZ=motion.centerZ+Math.cos(theta)*motion.radius+.3*Math.cos(time*.91+motion.wanderPhase);
    const progress=motion.transition>0?Math.min(1,elapsed/motion.transition):1;
    const blend=progress*progress*(3-2*progress);
    const blendSpeed=motion.transition>0&&progress<1?6*progress*(1-progress)/motion.transition:0;
    const vx=blendSpeed*(targetX-motion.fromX)+blend*(Math.cos(theta)*motion.radius*motion.angularSpeed+.201*Math.cos(time*.67+motion.wanderPhase));
    const vz=blendSpeed*(targetZ-motion.fromZ)+blend*(-Math.sin(theta)*motion.radius*motion.angularSpeed-.273*Math.sin(time*.91+motion.wanderPhase));
    x=motion.fromX+(targetX-motion.fromX)*blend;
    z=motion.fromZ+(targetZ-motion.fromZ)*blend;
    if(Math.hypot(vx,vz)>1e-5)heading=Math.atan2(-vx,-vz);
    speed=motion.speed*(.9+.1*Math.sin(time*.8+motion.wanderPhase));
    if(animal.species==='dolphin'){
      const jump=(time+motion.jumpPhase)%18;
      if(jump>16){y=Math.sin((jump-16)*Math.PI/2)*1.7;speed=motion.speed*.75;}
    }
  }
  out.x=x;out.y=y;out.z=z;out.heading=heading;out.yaw=heading;out.speed=speed;
  out.age=motion.age+(animal.egg?0:elapsed);
  out.hatchIn=animal.egg?Math.max(0,motion.hatchAt-time):0;
  return out;
}

// Decoder entities are the baseline for later delta patches. Never animate
// those objects in place: menus, nearby targets and rendering use this copy.
export function projectAnimalSnapshot(snapshot) {
  return {...snapshot,animals:(snapshot.animals||[]).map(animal=>{
    const display={...animal};
    sampleAnimalMotion(animal,snapshot.time,display);
    return display;
  })};
}
