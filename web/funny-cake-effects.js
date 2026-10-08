// Keep the trip on the rendered world: HUD, input and game physics stay untouched.
export function createFunnyCakeEffects({canvas}){
  const style=canvas?.style;
  let base=null,observedRemaining=null,deadline=0,started=0,lastTime=0;
  const smooth=value=>{const t=Math.max(0,Math.min(1,value));return t*t*(3-2*t);};
  function restore(){if(base&&style){for(const [key,value] of Object.entries(base))style[key]=value;base=null;}}
  function reset(){restore();observedRemaining=null;deadline=0;started=0;}
  function update({remaining=0,time=0,reduced=false,enabled=true}={}){
    if(!style)return;
    if(!enabled||!Number.isFinite(remaining)||remaining<=0||!Number.isFinite(time)){reset();return;}
    // Time is monotonic in normal play, but tolerate a replaced render loop.
    if(time<lastTime)reset();lastTime=time;
    if(remaining!==observedRemaining){
      if(observedRemaining===null||deadline<=time)started=time;
      observedRemaining=remaining;deadline=time+remaining;
    }
    // Let the last received duration expire even if snapshots stop arriving.
    const left=Math.min(remaining,Math.max(0,deadline-time));
    if(left<=0){restore();return;}
    if(!base){base={filter:style.filter||'',transform:style.transform||'',transformOrigin:style.transformOrigin||'',willChange:style.willChange||''};}
    const age=time-started,strength=smooth(age/.9)*smooth(left/3);
    const hue=reduced?15*strength:strength*(88*Math.sin(age*.38)+38*Math.sin(age*.21));
    const saturation=1+(reduced?.14:.65)*strength;
    const originalFilter=base.filter&&base.filter!=='none'?base.filter+' ':'';
    style.filter=originalFilter+`hue-rotate(${hue.toFixed(3)}deg) saturate(${saturation.toFixed(3)})`;
    if(reduced){
      style.transform=base.transform;style.transformOrigin=base.transformOrigin;style.willChange=base.willChange;
    }else{
      // Each axis has at least 2.6% overscan, comfortably beyond the 0.35%
      // sway. No rotation/skew means no exposed corners at any aspect ratio.
      const x=.35*strength*Math.sin(age*.73),y=.3*strength*Math.sin(age*.57);
      const sx=1+strength*(.032+.006*Math.sin(age*.61)),sy=1+strength*(.032+.006*Math.cos(age*.49));
      const originalTransform=base.transform&&base.transform!=='none'?' '+base.transform:'';
      style.transform=`translate(${x.toFixed(4)}%,${y.toFixed(4)}%) scale(${sx.toFixed(5)},${sy.toFixed(5)})`+originalTransform;
      style.transformOrigin='50% 50%';style.willChange='filter, transform';
    }
  }
  return {update,dispose:reset};
}
