// Positions are rendered between confirmed server samples, never beyond them.
// A stable clock anchor keeps packet-arrival jitter out of the movement speed.
export const LOCAL_MOTION_DELAY_MS=120;
export const REMOTE_MOTION_DELAY_MS=150;

export function createMotionClock({maxGapMs=1000,maxClockErrorMs=250}={}) {
  let anchorTime=0,anchorArrival=0,latestTime=null,latestArrival=0,generation=0;
  return {
    accept(time,arrival,{reset=false}={}) {
      if(!Number.isFinite(time)||!Number.isFinite(arrival))return false;
      if(!reset&&latestTime!==null&&(time<latestTime||arrival<latestArrival))return false;
      if(reset||latestTime===null||arrival-latestArrival>maxGapMs||Math.abs((arrival-anchorArrival)-(time-anchorTime))>maxClockErrorMs){
        anchorTime=time;anchorArrival=arrival;generation++;
      }
      latestTime=time;latestArrival=arrival;return true;
    },
    time(now,delay=LOCAL_MOTION_DELAY_MS) {
      return latestTime===null?0:anchorTime+Math.max(0,now-anchorArrival)-delay;
    },
    reset(){latestTime=null;generation++;},
    get generation(){return generation;},
    get latestTime(){return latestTime;},
  };
}

const angleDelta=(from,to)=>Math.atan2(Math.sin(to-from),Math.cos(to-from));

export function createMotionTrack({maxSamples=12,teleportDistance=15,maxIntervalMs=250}={}) {
  const samples=[];
  let identity,generation;
  const copy=(sample,out)=>{out.x=sample.x;out.y=sample.y;out.z=sample.z;out.yaw=sample.yaw;return out;};
  return {
    // key is an instance/posture/egg identity, not sprint or jump state. Normal
    // speed changes and jump arcs must continue through the same timeline.
    push(time,pose,{key='',epoch=0,reset=false}={}) {
      if(!Number.isFinite(time)||![pose.x,pose.y??0,pose.z].every(Number.isFinite))return false;
      const last=samples.at(-1),changed=reset||key!==identity||epoch!==generation;
      if(last&&!changed&&time<last.time)return false;
      const discontinuity=changed||!last||time-last.time>maxIntervalMs||Math.hypot(pose.x-last.x,(pose.y??0)-last.y,pose.z-last.z)>teleportDistance;
      if(discontinuity)samples.length=0;
      identity=key;generation=epoch;
      const sample={time,x:pose.x,y:pose.y??0,z:pose.z,yaw:Number.isFinite(pose.yaw)?pose.yaw:(last?.yaw??0)};
      if(samples.at(-1)?.time===time)samples[samples.length-1]=sample;else samples.push(sample);
      if(samples.length>maxSamples)samples.splice(0,samples.length-maxSamples);
      return discontinuity;
    },
    sample(time,out={}) {
      if(!samples.length)return null;
      if(time<=samples[0].time)return copy(samples[0],out);
      const last=samples.at(-1);
      if(time>=last.time)return copy(last,out);
      for(let i=1;i<samples.length;i++){
        const a=samples[i-1],b=samples[i];
        if(time>b.time)continue;
        const t=(time-a.time)/(b.time-a.time);
        out.x=a.x+(b.x-a.x)*t;out.y=a.y+(b.y-a.y)*t;out.z=a.z+(b.z-a.z)*t;
        out.yaw=a.yaw+angleDelta(a.yaw,b.yaw)*t;return out;
      }
      return copy(last,out);
    },
    reset(){samples.length=0;identity=undefined;generation=undefined;},
    get size(){return samples.length;},
  };
}
