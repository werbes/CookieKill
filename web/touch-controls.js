const DEAD_ZONE=.12;
const touchPointer=event=>event.pointerType==='touch'||event.pointerType==='pen';

// Pointer media queries describe input hardware, so a touchscreen PC can match
// the same queries as a phone. Keep mobile controls limited to mobile platforms.
export function isPhoneOrTablet(device={}){
  const userAgent=device.userAgent||'',platform=[device.platform,device.userAgentData?.platform].filter(Boolean).join(' ');
  const identity=platform+' '+userAgent;
  if(/\b(?:Windows|Win32|Win64|WinCE|CrOS|Chrome OS)\b/i.test(identity))return false;
  if(/\b(?:Android|iPhone|iPad|iPod|iOS)\b/i.test(identity))return true;
  // iPadOS can identify itself as desktop Safari on a Mac.
  return (/\bMacIntel\b/i.test(platform)||/\bMacintosh\b/i.test(userAgent))&&device.maxTouchPoints>1;
}

// Each finger owns just one control. Pointer capture keeps held inputs reliable
// when a finger leaves its button; document listeners also cover capture failure.
export function createTouchControls({root,joystick,stick,sprintButton,jumpButton,lookSurface,onMove=()=>{},onLook=()=>{},onAction=()=>{},onActivate=()=>{}}){
  let enabled=false,active=false,movePointer=null,lookPointer=null,jumpPointer=null;
  let forward=0,side=0,sprint=false,jump=false;
  const touchButtons=new Set();
  const eventRoot=root.ownerDocument||lookSurface.ownerDocument||lookSurface;
  const acceptsInput=()=>enabled&&active;
  const owned=id=>[movePointer,lookPointer,jumpPointer].some(pointer=>pointer?.id===id);
  const movement=()=>acceptsInput()?{forward,side,sprint,jump}:{forward:0,side:0,sprint:false,jump:false};
  function show(){root.hidden=!acceptsInput();}
  function paintStick(x=0,y=0){stick.style.transform=`translate(${x}px, ${y}px)`;}
  function paintButtons(){sprintButton.setAttribute('aria-pressed',String(sprint));jumpButton.setAttribute('aria-pressed',String(jump));}
  function capture(element,event){
    try{element.setPointerCapture?.(event.pointerId);}catch{}
    return {id:event.pointerId,element};
  }
  function release(pointer){
    if(!pointer)return;
    try{pointer.element.releasePointerCapture?.(pointer.id);}catch{}
  }
  function activate(event){if(!touchPointer(event))return false;onActivate();return acceptsInput();}
  function beginButton(button,event,run){
    if(!touchPointer(event)){touchButtons.delete(button);return;}
    const ready=activate(event);touchButtons.add(button);event.preventDefault();
    // A second finger does not reliably produce click while another finger is
    // moving. Act on its press, then discard any compatibility click it creates.
    if(ready&&!button.disabled)run();
  }
  function acceptClick(button,event){
    // Keyboard/assistive activation remains independent of pointer gestures.
    if(event.detail===0&&!touchPointer(event))return true;
    return !touchPointer(event)&&!event.sourceCapabilities?.firesTouchEvents&&!touchButtons.has(button);
  }
  function updateJoystick(event){
    const dx=event.clientX-movePointer.x,dy=event.clientY-movePointer.y,distance=Math.hypot(dx,dy),radius=movePointer.radius;
    if(!Number.isFinite(distance))return;
    const length=Math.min(1,distance/radius),amount=length<=DEAD_ZONE?0:(length-DEAD_ZONE)/(1-DEAD_ZONE),scale=distance?amount/distance:0;
    side=dx*scale||0;forward=-dy*scale||0;
    const visualScale=distance?Math.min(radius,distance)/distance:0;
    paintStick(dx*visualScale,dy*visualScale);
  }
  joystick.addEventListener('pointerdown',event=>{
    if(!activate(event)||movePointer||owned(event.pointerId))return;
    event.preventDefault();
    const rect=joystick.getBoundingClientRect();
    movePointer={...capture(joystick,event),x:rect.left+rect.width/2,y:rect.top+rect.height/2,radius:Math.max(1,Math.min(rect.width,rect.height)/2)};
    updateJoystick(event);onMove();
  });
  lookSurface.addEventListener('pointerdown',event=>{
    if(!activate(event)||lookPointer||owned(event.pointerId))return;
    event.preventDefault();lookPointer={...capture(lookSurface,event),x:event.clientX,y:event.clientY};
  });
  jumpButton.addEventListener('pointerdown',event=>{
    if(!activate(event)||jumpButton.disabled||jumpPointer||owned(event.pointerId))return;
    event.preventDefault();jumpPointer=capture(jumpButton,event);jump=true;paintButtons();onMove();
  });
  function toggleSprint(){sprint=!sprint;paintButtons();onMove();}
  sprintButton.addEventListener('pointerdown',event=>beginButton(sprintButton,event,toggleSprint));
  sprintButton.addEventListener('click',event=>{
    if(!acceptClick(sprintButton,event)||!acceptsInput()||sprintButton.disabled)return;
    toggleSprint();
  });
  root.addEventListener('pointerdown',event=>{
    const button=event.target.closest?.('[data-touch-action]');
    if(button&&root.contains(button)&&button!==jumpButton&&button!==sprintButton)beginButton(button,event,()=>onAction(button.dataset.touchAction));
  });
  root.addEventListener('click',event=>{
    const button=event.target.closest?.('[data-touch-action]');
    if(!button||!root.contains(button)||button===sprintButton||button===jumpButton||!acceptClick(button,event)||!acceptsInput()||button.disabled)return;
    onAction(button.dataset.touchAction);
  });
  eventRoot.addEventListener('pointermove',event=>{
    if(!acceptsInput()||!touchPointer(event))return;
    if(movePointer?.id===event.pointerId){
      event.preventDefault();const wasMoving=!!(forward||side);updateJoystick(event);
      // The game sends movement on its regular tick. Send dead-zone transitions
      // immediately, so starting and stopping never wait on that timer.
      if(wasMoving!==!!(forward||side))onMove();
    }
    if(lookPointer?.id===event.pointerId){
      event.preventDefault();const dx=event.clientX-lookPointer.x,dy=event.clientY-lookPointer.y;
      if(Number.isFinite(dx)&&Number.isFinite(dy)){lookPointer.x=event.clientX;lookPointer.y=event.clientY;if(dx||dy)onLook(dx,dy);}
    }
  },{passive:false});
  function endPointer(event){
    let changed=false;
    if(movePointer?.id===event.pointerId){const pointer=movePointer;movePointer=null;forward=0;side=0;paintStick();release(pointer);changed=true;}
    if(lookPointer?.id===event.pointerId){const pointer=lookPointer;lookPointer=null;release(pointer);}
    if(jumpPointer?.id===event.pointerId){const pointer=jumpPointer;jumpPointer=null;jump=false;paintButtons();release(pointer);changed=true;}
    if(changed)onMove();
  }
  for(const type of ['pointerup','pointercancel'])eventRoot.addEventListener(type,endPointer);
  for(const element of [joystick,lookSurface,jumpButton])element.addEventListener('lostpointercapture',endPointer);
  function reset(){
    const pointers=[movePointer,lookPointer,jumpPointer];movePointer=null;lookPointer=null;jumpPointer=null;
    forward=0;side=0;sprint=false;jump=false;paintStick();paintButtons();
    for(const pointer of pointers)release(pointer);
    onMove();
  }
  paintStick();paintButtons();show();
  return {
    setEnabled(value){value=!!value;if(value===enabled)return;enabled=value;if(!enabled)reset();show();},
    setActive(value){value=!!value;if(value===active)return;active=value;if(!active)reset();show();},
    reset,movement,
  };
}
